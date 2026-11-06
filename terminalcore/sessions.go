// Package terminalcore keeps terminal jobs outside the Agent connection and unit.
package terminalcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/creack/pty"
)

type Session struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
	Attached  int    `json:"attached"`
}
type Manager struct {
	Binary      string
	Socket      string
	Unit        string
	Directory   string
	prepareMu   sync.Mutex
	mutationMu  sync.Mutex
	noStartOnce sync.Once
	noStart     bool
}

var Default = &Manager{Socket: "/run/ppeelink-terminal/tmux.sock", Unit: "ppeelink-terminal.service", Directory: "/run/ppeelink-terminal"}
var sessionID = regexp.MustCompile(`^[a-f0-9]{24}$`)

func (m *Manager) Prepare(ctx context.Context, install bool) error {
	for !m.prepareMu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	defer m.prepareMu.Unlock()
	tmux, err := exec.LookPath(m.binaryName())
	if err != nil && install {
		if err = installTmux(ctx); err != nil {
			return err
		}
		tmux, err = exec.LookPath(m.binaryName())
	}
	if err != nil {
		return errors.New("未启用任务保护：服务器缺少 tmux，请安装后重试")
	}
	if _, err = exec.LookPath("systemctl"); err != nil {
		return errors.New("任务保护需要 systemd")
	}
	if exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", m.Unit).Run() != nil {
		if err = os.MkdirAll(m.Directory, 0700); err != nil {
			return err
		}
		// Forking mode also works on Ubuntu 20.04's tmux 3.0a (-D is newer).
		pidFile := filepath.Join(m.Directory, "tmux.pid")
		config := filepath.Join(m.Directory, "tmux.conf")
		pidCommand := "printf '%s\\n' '#{pid}' > " + shellQuote(pidFile)
		contents := "set-option -s exit-empty off\nrun-shell " + strconv.Quote(pidCommand) + "\n"
		if err := os.WriteFile(config, []byte(contents), 0600); err != nil {
			return err
		}
		_ = exec.CommandContext(ctx, "systemctl", "stop", m.Unit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "reset-failed", m.Unit).Run()
		_ = os.Remove(pidFile)
		args := []string{"--quiet", "--collect", "--unit=" + m.Unit, "--property=Type=forking", "--property=PIDFile=" + pidFile, "--property=TimeoutStartSec=10", "--property=Restart=on-failure", "--property=UMask=0077", tmux, "-S", m.Socket, "-f", config, "start-server"}
		out, startErr := exec.CommandContext(ctx, "systemd-run", args...).CombinedOutput()
		if startErr != nil {
			return fmt.Errorf("无法启动独立终端服务：%s", strings.TrimSpace(string(out)))
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		// Read-only probes cannot start a server; modern clients additionally use -N.
		if _, err = m.command(ctx, "show-options", "-g", "exit-empty"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("独立终端服务尚未就绪")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(40 * time.Millisecond):
		}
	}
	for _, option := range [][]string{{"history-limit", "5000"}, {"prefix", "None"}, {"prefix2", "None"}} {
		if _, err = m.command(ctx, append([]string{"set-option", "-g"}, option...)...); err != nil {
			return err
		}
	}
	return nil
}
func installTmux(ctx context.Context) error {
	var commands [][]string
	switch {
	case commandExists("apt-get"):
		commands = [][]string{{"apt-get", "update"}, {"apt-get", "install", "-y", "--no-install-recommends", "tmux"}}
	case commandExists("dnf"):
		commands = [][]string{{"dnf", "install", "-y", "tmux"}}
	case commandExists("yum"):
		commands = [][]string{{"yum", "install", "-y", "tmux"}}
	default:
		return errors.New("未启用任务保护：请通过系统包管理器安装 tmux")
	}
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		if err := cmd.Run(); err != nil {
			return errors.New("tmux 安装失败，请检查软件源或包管理器锁后重试")
		}
	}
	return nil
}
func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
func (m *Manager) command(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.binaryName(), append(m.clientArgs(), args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("终端会话操作失败：%s", strings.TrimSpace(string(out)))
	}
	return out, nil
}
func (m *Manager) target(owner int, id string) string { return "p" + strconv.Itoa(owner) + "_" + id }
func validOwnerID(owner int, id string) bool          { return owner > 0 && sessionID.MatchString(id) }
func (m *Manager) List(ctx context.Context, owner int) ([]Session, error) {
	if owner <= 0 {
		return nil, errors.New("无效的会话所有者")
	}
	if err := m.Prepare(ctx, false); err != nil {
		return nil, err
	}
	return m.list(ctx, owner)
}
func (m *Manager) list(ctx context.Context, owner int) ([]Session, error) {
	out, err := m.command(ctx, "list-sessions", "-F", "#{session_name}\t#{session_created}\t#{session_attached}\t#{@ppeelink-name}")
	list := []Session{}
	if err != nil {
		// A healthy server with no sessions is not an unavailable server.
		if _, e := m.command(ctx, "show-options", "-g", "exit-empty"); e == nil && strings.Contains(err.Error(), "no sessions") {
			return list, nil
		}
		return nil, err
	}
	prefix := "p" + strconv.Itoa(owner) + "_"
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.SplitN(line, "\t", 4)
		if len(f) < 3 || !strings.HasPrefix(f[0], prefix) {
			continue
		}
		id := strings.TrimPrefix(f[0], prefix)
		if !sessionID.MatchString(id) {
			continue
		}
		created, _ := strconv.ParseInt(f[1], 10, 64)
		attached, _ := strconv.Atoi(f[2])
		name := "终端会话"
		if len(f) == 4 && f[3] != "" {
			name = f[3]
		}
		list = append(list, Session{ID: id, Name: name, CreatedAt: created, Attached: attached})
	}
	return list, nil
}
func (m *Manager) Create(ctx context.Context, owner int, name string) (Session, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "终端会话"
	}
	if owner <= 0 || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 64 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Session{}, errors.New("会话名称最长 64 字，不能包含控制字符")
	}
	if err := m.Prepare(ctx, false); err != nil {
		return Session{}, err
	}
	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()
	list, err := m.list(ctx, owner)
	if err != nil {
		return Session{}, err
	}
	if len(list) >= 16 {
		return Session{}, errors.New("每个账户最多保留 16 个会话，请先结束不用的会话")
	}
	all, err := m.command(ctx, "list-sessions", "-F", "#{session_name}")
	if err == nil && len(strings.Fields(string(all))) >= 64 {
		return Session{}, errors.New("服务器会话已达上限")
	}
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return Session{}, err
	}
	id := hex.EncodeToString(b)
	target := m.target(owner, id)
	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	dir := "/root"
	if _, err := os.Stat(dir); err != nil {
		dir = "/"
	}
	_, err = m.command(ctx, "new-session", "-d", "-s", target, "-x", "100", "-y", "30", "-c", dir, shell)
	if err != nil {
		return Session{}, err
	}
	// Legacy tmux may auto-start on new-session. Never advertise such a server
	// as protected unless its PID belongs to our independent systemd unit.
	if err := m.serverOwned(ctx, target); err != nil {
		m.command(ctx, "kill-session", "-t", "="+target)
		return Session{}, err
	}
	if _, err = m.command(ctx, "set-option", "-t", target, "@ppeelink-name", name); err != nil {
		m.command(ctx, "kill-session", "-t", target)
		return Session{}, err
	}
	return Session{ID: id, Name: name, CreatedAt: time.Now().Unix()}, nil
}
func (m *Manager) Exists(ctx context.Context, owner int, id string) error {
	if !validOwnerID(owner, id) {
		return errors.New("会话不存在或无权访问")
	}
	if _, err := m.command(ctx, "has-session", "-t", "="+m.target(owner, id)); err != nil {
		return errors.New("会话已结束，请新建会话")
	}
	return nil
}
func (m *Manager) Terminate(ctx context.Context, owner int, id string) error {
	if err := m.Exists(ctx, owner, id); err != nil {
		return err
	}
	_, err := m.command(ctx, "kill-session", "-t", "="+m.target(owner, id))
	return err
}

type Attachment struct {
	File *os.File
	Cmd  *exec.Cmd
	once sync.Once
}

func (p *Attachment) Close() { p.once.Do(func() { _ = p.Cmd.Process.Kill(); _ = p.File.Close() }) }
func (m *Manager) Attach(ctx context.Context, owner int, id string, cols, rows int) (*Attachment, error) {
	if err := m.Exists(ctx, owner, id); err != nil {
		return nil, err
	}
	if cols < 10 || cols > 500 {
		cols = 100
	}
	if rows < 3 || rows > 300 {
		rows = 30
	}
	cmd := exec.Command(m.binaryName(), append(m.clientArgs(), "attach-session", "-t", "="+m.target(owner, id))...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	cmd.Dir = filepath.Dir(m.Socket)
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	attachment := &Attachment{File: file, Cmd: cmd}
	// Older clients flush pre-attach input while entering raw mode. Do not report
	// readiness until this client is attached to the requested session.
	readyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	want := strconv.Itoa(cmd.Process.Pid) + "\t" + m.target(owner, id)
	for {
		out, err := m.command(readyCtx, "list-clients", "-F", "#{client_pid}\t#{session_name}")
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line == want {
					return attachment, nil
				}
			}
		}
		select {
		case <-readyCtx.Done():
			attachment.Close()
			_ = cmd.Wait()
			return nil, errors.New("会话连接未就绪，请重试")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (m *Manager) binaryName() string {
	if m.Binary != "" {
		return m.Binary
	}
	return "tmux"
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func (m *Manager) clientArgs() []string {
	m.noStartOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		m.noStart = exec.CommandContext(ctx, m.binaryName(), "-N", "-V").Run() == nil
	})
	args := []string{"-S", m.Socket, "-f", "/dev/null"}
	if m.noStart {
		return append([]string{"-N"}, args...)
	}
	return args
}
func (m *Manager) serverOwned(ctx context.Context, target string) error {
	out, err := m.command(ctx, "display-message", "-p", "-t", target+":", "#{pid}")
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return errors.New("终端服务 PID 无效")
	}
	group, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil || !strings.Contains(string(group), "/"+m.Unit) {
		return errors.New("终端服务未处于独立服务中，请稍后重试")
	}
	return nil
}
