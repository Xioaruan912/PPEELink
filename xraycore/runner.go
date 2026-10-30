package xraycore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

// Runner 管理一台机器上的 xray 进程。有 systemd 且以 root 运行时托管为独立服务
// （面板或 agent 重启不会断开用户连接），否则作为子进程运行。
type Runner struct {
	certificates  map[string]string
	Dir           string
	Unit          string // systemd 服务名
	mu            sync.Mutex
	systemd       bool
	cmd           *exec.Cmd
	exited        chan struct{}
	rev           string
	apiPort       int
	lastErr       string
	stderr        *tailBuffer
	stopping      bool
	website       *WebsiteCollector
	websiteStats  *websiteCounters
	egressLookup  egressLookup
	collectMu     sync.Mutex
	statsConn     *grpc.ClientConn
	statsPort     int
	statusChecked time.Time
	statusRunning bool
	versionStamp  string
	versionCached string
}

func NewRunner(dir, unit string) *Runner {
	// 子进程以 Dir 为工作目录启动，二进制与配置路径必须是绝对路径
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	_, err := os.Stat("/run/systemd/system")
	return &Runner{
		Dir:     dir,
		Unit:    unit,
		systemd: err == nil && os.Geteuid() == 0 && os.Getenv("PPEELINK_XRAY_SYSTEMD") != "0",
		stderr:  &tailBuffer{max: 4096},
	}
}

func (r *Runner) bin() string        { return filepath.Join(r.Dir, "xray") }
func (r *Runner) configPath() string { return filepath.Join(r.Dir, "config.json") }

// Apply 让 xray 达到 spec 描述的状态；配置未变且进程在跑时什么也不做。
func (r *Runner) Apply(ctx context.Context, spec Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rev := spec.Rev()
	if !spec.Enabled || len(spec.Inbounds) == 0 {
		r.stopLocked()
		r.stopWebsiteLocked()
		if r.runningLocked() {
			r.lastErr = "xray did not stop"
			return fmt.Errorf("%s", r.lastErr)
		}
		r.rev, r.apiPort, r.lastErr = rev, 0, ""
		return nil
	}
	wantsWebsite := spec.WebsiteTraffic != nil && spec.WebsiteTraffic.Enabled
	if rev == r.rev && r.runningLocked() && (!wantsWebsite || r.website != nil) {
		r.lastErr = ""
		return nil
	}
	// Keep the desired revision above independent of DNS answers. A healthy,
	// unchanged config never re-resolves/restarts on the periodic Agent tick.
	resolved, err := prepareEgress(ctx, spec, r.egressLookup)
	if err != nil {
		r.lastErr = err.Error()
		return err
	}
	spec = resolved
	if _, err := EnsureXray(ctx, r.Dir, spec.Version); err != nil {
		r.lastErr = err.Error()
		return err
	}
	prepared, certificates, err := prepareAutoTLS(r.Dir, spec)
	if err != nil {
		r.lastErr = err.Error()
		return err
	}
	spec = prepared
	var nextWebsite *WebsiteCollector
	if spec.WebsiteTraffic != nil && spec.WebsiteTraffic.Enabled {
		if r.websiteStats == nil {
			r.websiteStats = newWebsiteCounters()
		}
		var err error
		nextWebsite, err = newWebsiteCollector(spec, r.websiteStats)
		if err != nil {
			r.lastErr = err.Error()
			return err
		}
	}
	committed := false
	defer func() {
		if !committed && nextWebsite != nil {
			_ = nextWebsite.Close()
		}
	}()
	cfg, err := BuildConfigWithWebsiteCollector(spec, nextWebsite)
	if err != nil {
		r.lastErr = err.Error()
		return err
	}
	// 先用 xray 自检新配置，通过后再替换，失败时继续运行旧配置
	candidate := filepath.Join(r.Dir, "config.next.json")
	if err := os.WriteFile(candidate, cfg, 0o600); err != nil {
		return err
	}
	if out, err := exec.CommandContext(ctx, r.bin(), "run", "-test", "-c", candidate).CombinedOutput(); err != nil {
		r.lastErr = "配置自检失败: " + lastLines(string(out), 3)
		return fmt.Errorf("%s", r.lastErr)
	}
	if err := os.Rename(candidate, r.configPath()); err != nil {
		return err
	}
	if err := r.restartLocked(); err != nil {
		r.lastErr = err.Error()
		return err
	}
	r.stopWebsiteLocked()
	r.website = nextWebsite
	r.certificates = certificates
	committed = true
	r.rev, r.apiPort, r.lastErr = rev, spec.APIPort, ""
	return nil
}

// Rev 当前已应用的期望状态摘要。
func (r *Runner) Rev() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rev
}

// Status 运行状态与最近一次错误（含进程退出时的日志尾部）。
func (r *Runner) Status() (running bool, lastErr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	running = r.runningLocked()
	lastErr = r.lastErr
	if !running && lastErr == "" && r.rev != "" && r.apiPort != 0 {
		lastErr = "xray 已退出: " + lastLines(r.stderr.String(), 3)
	}
	return
}

func (r *Runner) runningLocked() bool {
	if r.systemd {
		if !r.statusChecked.IsZero() && time.Since(r.statusChecked) < 2*time.Second {
			return r.statusRunning
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		r.statusRunning = exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", r.Unit).Run() == nil
		r.statusChecked = time.Now()
		return r.statusRunning
	}
	if r.cmd == nil || r.exited == nil {
		return false
	}
	select {
	case <-r.exited:
		return false
	default:
		return true
	}
}

func (r *Runner) restartLocked() error {
	r.closeStatsLocked()
	r.statusChecked = time.Time{}
	defer func() { r.statusChecked = time.Time{} }()
	if r.systemd {
		unit := fmt.Sprintf(`[Unit]
Description=Xray managed by PPEELink
After=network-online.target

[Service]
ExecStart=%s run -c %s
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
NoNewPrivileges=true
WorkingDirectory=%s

[Install]
WantedBy=multi-user.target
`, r.bin(), r.configPath(), r.Dir)
		path := "/etc/systemd/system/" + r.Unit + ".service"
		if old, _ := os.ReadFile(path); string(old) != unit {
			if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
				return err
			}
			_ = exec.Command("systemctl", "daemon-reload").Run()
			_ = exec.Command("systemctl", "enable", r.Unit).Run()
		}
		if out, err := exec.Command("systemctl", "restart", r.Unit).CombinedOutput(); err != nil {
			return fmt.Errorf("启动 xray 服务失败: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	r.stopLocked()
	r.stderr.Reset()
	cmd := exec.Command(r.bin(), "run", "-c", r.configPath())
	cmd.Dir = r.Dir
	cmd.Stdout = r.stderr
	cmd.Stderr = r.stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	r.cmd, r.exited = cmd, exited
	// 端口占用等启动错误会让 xray 立即退出，稍等片刻再判断
	select {
	case <-exited:
		return fmt.Errorf("xray 启动失败: %s", lastLines(r.stderr.String(), 3))
	case <-time.After(800 * time.Millisecond):
		return nil
	}
}

func (r *Runner) stopLocked() {
	r.closeStatsLocked()
	r.statusChecked = time.Time{}
	defer func() { r.statusChecked = time.Time{} }()
	if r.systemd {
		_ = exec.Command("systemctl", "stop", r.Unit).Run()
		return
	}
	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Signal(os.Interrupt)
		select {
		case <-r.exited:
		case <-time.After(3 * time.Second):
			_ = r.cmd.Process.Kill()
			<-r.exited
		}
	}
	r.cmd, r.exited = nil, nil
}

// Stop stops the agent-owned relay as well as Xray. Without a relay, systemd
// mode retains its historical keep-running behavior; with collection enabled,
// Xray must stop too because its authenticated egress endpoint is agent-owned.
func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeStatsLocked()
	if !r.systemd || r.website != nil {
		r.stopLocked()
	}
	r.stopWebsiteLocked()
}

func (r *Runner) stopWebsiteLocked() {
	if r.website != nil {
		_ = r.website.Close()
		r.website = nil
	}
}

// CollectWebsiteTraffic drains successfully forwarded payload counters, even
// after disable/Stop/Uninstall. The caller owns retry/persistence of the batch.
// This does not perform Xray API calls and does not include agent/monitor traffic.
func (r *Runner) CollectWebsiteTraffic() []WebsiteTrafficBucket {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.websiteStats == nil {
		return nil
	}
	return r.websiteStats.drain()
}

// Uninstall 停止 xray 并删除 systemd 服务、二进制与配置目录。
func (r *Runner) Uninstall() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
	r.stopWebsiteLocked()
	if r.runningLocked() {
		return fmt.Errorf("xray did not stop; refusing uninstall")
	}
	if r.systemd {
		_ = exec.Command("systemctl", "disable", "--now", r.Unit).Run()
		path := "/etc/systemd/system/" + r.Unit + ".service"
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		_ = exec.Command("systemctl", "daemon-reload").Run()
	}
	r.rev, r.apiPort, r.lastErr = "", 0, ""
	return removeManagedDir(r.Dir)
}

// removeManagedDir 只删除确实由我们创建的目录（含 xray 或 config.json），防止误删。
func removeManagedDir(dir string) error {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) || strings.Count(dir, string(filepath.Separator)) < 2 {
		return fmt.Errorf("拒绝删除目录 %s", dir)
	}
	managed := false
	for _, name := range []string{"xray", "config.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			managed = true
		}
	}
	if !managed {
		return nil
	}
	return os.RemoveAll(dir)
}

// parseOnlineUsers 解析 statsgetallonlineusers 输出：{"users":["user>>>u1>>>online",...]}。
func parseOnlineUsers(out []byte) []string {
	var resp struct {
		Users []string `json:"users"`
	}
	if json.Unmarshal(out, &resp) != nil {
		return nil
	}
	emails := []string{}
	for _, u := range resp.Users {
		parts := strings.Split(u, ">>>")
		if len(parts) >= 2 {
			emails = append(emails, parts[1])
		} else if u != "" {
			emails = append(emails, u)
		}
	}
	return emails
}

// countOnlineIPs 解析 statsonlineiplist 输出：{"name":"...","ips":{"1.2.3.4":时间戳}}。
func countOnlineIPs(out []byte) int {
	var resp struct {
		IPs map[string]any `json:"ips"`
	}
	if json.Unmarshal(out, &resp) != nil {
		return 0
	}
	return len(resp.IPs)
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, " | "))
}

// tailBuffer 只保留最后 max 字节的输出。
type tailBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf.Write(p)
	if over := t.buf.Len() - t.max; over > 0 {
		t.buf.Next(over)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

func (t *tailBuffer) Reset() {
	t.mu.Lock()
	t.buf.Reset()
	t.mu.Unlock()
}

// Version only executes the binary again after its file identity changes.
func (r *Runner) Version() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, err := os.Stat(r.bin())
	if err != nil {
		r.versionStamp = ""
		r.versionCached = ""
		return ""
	}
	stamp := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	if stamp != r.versionStamp {
		r.versionCached = InstalledVersion(r.Dir)
		r.versionStamp = stamp
	}
	return r.versionCached
}
