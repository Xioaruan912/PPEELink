package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sagernet/quic-go"
)

const agentVersion = "1.6"

type probeNode struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme,omitempty"`
	SNI    string `json:"sni,omitempty"`
}

type waitEnvelope struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		JobID      string      `json:"jobId"`
		Action     string      `json:"action"`
		TargetHost string      `json:"targetHost,omitempty"`
		SpeedMode  string      `json:"speedMode,omitempty"`
		Nodes      []probeNode `json:"nodes"`
	} `json:"data"`
}

type probeResult struct {
	NodeID    int    `json:"nodeId"`
	Kind      string `json:"kind"`
	Success   bool   `json:"success"`
	RTT       int    `json:"rtt"`
	Error     string `json:"error,omitempty"`
	IPVersion int    `json:"ipVersion,omitempty"`
}

type reportPayload struct {
	JobID   string        `json:"jobId,omitempty"`
	Results []probeResult `json:"results"`
}

type ackPayload struct {
	JobID  string `json:"jobId"`
	Action string `json:"action"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

func main() {
	panel := flag.String("panel", "", "PPEELink 面板地址，例如 https://panel.example.com")
	token := flag.String("token", "", "用户 Agent 令牌")
	concurrency := flag.Int("concurrency", 4, "单次测速最大并发数（默认 4，避免网络滥用）")
	timeout := flag.Duration("timeout", 4*time.Second, "单个节点超时时间")
	once := flag.Bool("once", false, "仅立即拉取测速一次后退出")
	install := flag.Bool("install", false, "安装到后台常驻并设置开机自启")
	daemon := flag.Bool("daemon", false, "后台模式（自动读取已保存配置）")
	uninstall := flag.Bool("uninstall", false, "无痕卸载并清除所有本地文件与自启项")
	flag.Parse()

	cleanupOldBinary()

	if *uninstall {
		performTracelessUninstall()
		return
	}

	panelURL := strings.TrimRight(strings.TrimSpace(*panel), "/")
	tok := strings.TrimSpace(*token)
	conc := *concurrency

	if *daemon && (panelURL == "" || tok == "") {
		if cfg, err := loadSavedConfig(); err == nil {
			if panelURL == "" {
				panelURL = strings.TrimRight(strings.TrimSpace(cfg.Panel), "/")
			}
			if tok == "" {
				tok = strings.TrimSpace(cfg.Token)
			}
			if cfg.Concurrency > 0 {
				conc = cfg.Concurrency
			}
		}
	}

	if conc < 1 {
		conc = 1
	}
	if conc > 8 {
		conc = 8
	}

	if *install {
		if panelURL == "" || tok == "" {
			fmt.Fprintf(os.Stderr, "用法: %s -install -panel https://your-panel.com -token <TOKEN>\n", os.Args[0])
			os.Exit(2)
		}
		if err := installBackground(panelURL, tok, conc); err != nil {
			fmt.Fprintf(os.Stderr, "安装失败: %v\n", err)
			os.Exit(1)
		}
		dir, _ := installDir()
		cfgFile, _ := configPath()
		fmt.Println("==========================================")
		fmt.Printf("  PPEELink 用户 Agent v%s 安装成功\n", agentVersion)
		fmt.Println("==========================================")
		fmt.Printf("  运行状态 : 已在后台静默启动（开机自启）\n")
		fmt.Printf("  安装目录 : %s\n", dir)
		fmt.Printf("  配置文件 : %s\n", cfgFile)
		fmt.Printf("  面板地址 : %s\n", panelURL)
		fmt.Println("  管理方式 : 可在面板「用户 Agent」页面随时发起实测、远程更新或无痕卸载")
		fmt.Println("==========================================")
		return
	}

	if panelURL == "" || tok == "" {
		fmt.Fprintf(os.Stderr, "用法: %s -panel https://your-panel.com -token <TOKEN>\n", os.Args[0])
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Proxy: nil explicitly ignores HTTP/SOCKS system proxy settings.
	client := &http.Client{
		Timeout: 40 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
	log.SetFlags(log.Ltime)

	if *once {
		nodes, err := fetchJobs(ctx, client, panelURL, tok)
		if err != nil {
			log.Fatalf("[用户 Agent] 拉取失败: %v", err)
		}
		results := runProbes(ctx, nodes, conc, *timeout)
		if err := sendReport(ctx, client, panelURL, tok, "", results); err != nil {
			log.Fatalf("[用户 Agent] 回报失败: %v", err)
		}
		logSummary("", results)
		return
	}

	if !*daemon {
		log.Printf("[用户 Agent v%s] 已启动，按需待命中（空闲时不向任何节点发包）", agentVersion)
	}

	var paused atomic.Bool
	stopTray := startSystemTray(panelURL, &paused, stop)
	defer stopTray()

	// Warm up ISP detection in background on startup.
	_ = detectLocalNetEnv()

	backoff := 2 * time.Second
	connected := false
	for {
		if ctx.Err() != nil {
			return
		}
		jobID, action, targetHost, speedMode, nodes, err := waitForJob(ctx, client, panelURL, tok)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			connected = false
			if !*daemon {
				log.Printf("[用户 Agent] 连接面板失败: %v（%s 后重试）", err, backoff)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		if !connected {
			connected = true
			backoff = 2 * time.Second
			if !*daemon {
				log.Printf("[用户 Agent] 已连接面板，等待指令…")
			}
		}

		switch action {
		case "uninstall":
			_ = sendAck(ctx, client, panelURL, tok, ackPayload{JobID: jobID, Action: "uninstall", OK: true})
			stopTray()
			performTracelessUninstall()
			os.Exit(0)
		case "update":
			if err := performSelfUpdate(client, panelURL, tok); err != nil {
				_ = sendAck(ctx, client, panelURL, tok, ackPayload{JobID: jobID, Action: "update", OK: false, Error: err.Error()})
				continue
			}
			_ = sendAck(ctx, client, panelURL, tok, ackPayload{JobID: jobID, Action: "update", OK: true})
			stopTray()
			os.Exit(0)
		case "trace":
			if strings.TrimSpace(targetHost) == "" {
				continue
			}
			traceCtx, cancel := context.WithTimeout(ctx, 22*time.Second)
			hops, traceErr := runOutboundTrace(traceCtx, targetHost)
			cancel()
			payload := traceReportPayload{JobID: jobID, Host: targetHost, Hops: hops}
			if traceErr != nil {
				payload.Error = traceErr.Error()
			}
			_ = sendJSONReport(ctx, client, panelURL+"/api/v1/probe/trace-report", tok, payload)
		case "unlock":
			unlockCtx, cancel := context.WithTimeout(ctx, 24*time.Second)
			payload := runNativeUnlock(unlockCtx)
			cancel()
			payload.JobID = jobID
			_ = sendJSONReport(ctx, client, panelURL+"/api/v1/probe/unlock-report", tok, payload)
		case "speedtest":
			stCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			payload := runUserSpeedtest(stCtx, speedMode)
			cancel()
			payload.JobID = jobID
			_ = sendJSONReport(ctx, client, panelURL+"/api/v1/probe/speedtest-report", tok, payload)
		case "probe", "":
			if len(nodes) == 0 || paused.Load() {
				continue
			}
			results := runProbes(ctx, nodes, conc, *timeout)
			if err := sendReport(ctx, client, panelURL, tok, jobID, results); err != nil {
				continue
			}
			if !*daemon {
				logSummary(jobID, results)
			}
		}
	}
}

func waitForJob(ctx context.Context, client *http.Client, panelURL, token string) (string, string, string, string, []probeNode, error) {
	env := detectLocalNetEnv()
	q := url.Values{}
	q.Set("timeout", "25")
	q.Set("version", agentVersion)
	if env.HasIPv6 {
		q.Set("hasIpv6", "1")
	} else {
		q.Set("hasIpv6", "0")
	}
	if env.ISP != "" {
		q.Set("isp", env.ISP)
	}
	if env.IfaceName != "" {
		q.Set("iface", env.IfaceName)
	}
	if env.PublicIP != "" {
		q.Set("publicIp", env.PublicIP)
	}
	endpoint := fmt.Sprintf("%s/api/v1/probe/wait?%s", panelURL, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", "", "", nil, err
	}
	req.Header.Set("Authorization", "Probe "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", "", "", nil, errors.New("令牌无效或已停用")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", "", "", "", nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var envResp waitEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envResp); err != nil {
		return "", "", "", "", nil, err
	}
	return envResp.Data.JobID, envResp.Data.Action, envResp.Data.TargetHost, envResp.Data.SpeedMode, envResp.Data.Nodes, nil
}

func sendJSONReport(ctx context.Context, client *http.Client, endpoint, token string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Probe "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func fetchJobs(ctx context.Context, client *http.Client, panelURL, token string) ([]probeNode, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panelURL+"/api/v1/probe/jobs", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Probe "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var env waitEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	return env.Data.Nodes, nil
}

func sendReport(ctx context.Context, client *http.Client, panelURL, token, jobID string, results []probeResult) error {
	raw, err := json.Marshal(reportPayload{JobID: jobID, Results: results})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, panelURL+"/api/v1/probe/report", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Probe "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func sendAck(ctx context.Context, client *http.Client, panelURL, token string, ack ackPayload) error {
	raw, err := json.Marshal(ack)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, panelURL+"/api/v1/probe/ack", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Probe "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func runProbes(ctx context.Context, nodes []probeNode, concurrency int, timeout time.Duration) []probeResult {
	results := make([]probeResult, len(nodes))
	slots := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, target probeNode) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				results[idx] = probeResult{NodeID: target.ID, Kind: probeKind(target.Scheme), Success: false, RTT: -1, Error: "已取消"}
				return
			}
			results[idx] = probeOne(ctx, target, timeout)
		}(i, n)
	}
	wg.Wait()
	return results
}

func probeKind(scheme string) string {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "hysteria2", "hy2", "tuic":
		return "quic"
	default:
		return "tcp"
	}
}

func probeOne(ctx context.Context, target probeNode, timeout time.Duration) probeResult {
	kind := probeKind(target.Scheme)
	if target.Host == "" || target.Port <= 0 {
		return probeResult{NodeID: target.ID, Kind: kind, Success: false, RTT: -1, Error: "地址无效"}
	}
	if kind == "quic" {
		return probeQUIC(ctx, target, timeout)
	}
	return probeTCP(ctx, target, timeout)
}

func probeTCP(ctx context.Context, target probeNode, timeout time.Duration) probeResult {
	addr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
	wantV6 := targetIsIPv6(target.Host)
	var iface *physicalInterface
	if !isLoopbackHost(target.Host) {
		iface = findPhysicalInterface(wantV6)
	}
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(network, _ string, c syscall.RawConn) error {
			return bindSocketToInterface(network, c, iface)
		},
	}
	if iface != nil {
		if wantV6 && iface.IPv6 != nil {
			dialer.LocalAddr = &net.TCPAddr{IP: iface.IPv6}
		} else if !wantV6 && iface.IPv4 != nil {
			dialer.LocalAddr = &net.TCPAddr{IP: iface.IPv4}
		}
	}
	network := "tcp"
	if wantV6 {
		network = "tcp6"
	} else if iface != nil && iface.IPv4 != nil {
		network = "tcp4"
	}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return probeResult{NodeID: target.ID, Kind: "tcp", Success: false, RTT: -1, Error: shortErr(err)}
	}
	rtt := max(int(time.Since(start).Milliseconds()), 1)
	ipVer := ipVersionOf(conn.RemoteAddr())
	_ = conn.Close()
	return probeResult{NodeID: target.ID, Kind: "tcp", Success: true, RTT: rtt, IPVersion: ipVer}
}

func probeQUIC(ctx context.Context, target probeNode, timeout time.Duration) probeResult {
	wantV6 := targetIsIPv6(target.Host)
	var iface *physicalInterface
	if !isLoopbackHost(target.Host) {
		iface = findPhysicalInterface(wantV6)
	}
	udpNet := "udp4"
	localBind := "0.0.0.0:0"
	if wantV6 {
		udpNet = "udp6"
		localBind = "[::]:0"
		if iface != nil && iface.IPv6 != nil {
			localBind = net.JoinHostPort(iface.IPv6.String(), "0")
		}
	} else if iface != nil && iface.IPv4 != nil {
		localBind = net.JoinHostPort(iface.IPv4.String(), "0")
	}

	resolveCtx, cancelResolve := context.WithTimeout(ctx, timeout)
	defer cancelResolve()
	remoteAddr, err := net.ResolveUDPAddr(udpNet, net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)))
	if err != nil {
		return probeResult{NodeID: target.ID, Kind: "quic", Success: false, RTT: -1, Error: shortErr(err)}
	}

	lc := net.ListenConfig{
		Control: func(network, _ string, c syscall.RawConn) error {
			return bindSocketToInterface(network, c, iface)
		},
	}
	pc, err := lc.ListenPacket(resolveCtx, udpNet, localBind)
	if err != nil {
		return probeResult{NodeID: target.ID, Kind: "quic", Success: false, RTT: -1, Error: shortErr(err)}
	}
	defer pc.Close()

	sni := strings.TrimSpace(target.SNI)
	if sni == "" {
		sni = target.Host
	}
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         sni,
		NextProtos:         []string{"h3"},
	}
	quicConf := &quic.Config{
		HandshakeIdleTimeout: timeout,
		MaxIdleTimeout:       timeout,
	}

	dialCtx, cancelDial := context.WithTimeout(ctx, timeout)
	defer cancelDial()
	start := time.Now()
	conn, err := quic.Dial(dialCtx, pc, remoteAddr, tlsConf, quicConf)
	if err != nil {
		return probeResult{NodeID: target.ID, Kind: "quic", Success: false, RTT: -1, Error: shortErr(err)}
	}
	rtt := max(int(time.Since(start).Milliseconds()), 1)
	_ = conn.CloseWithError(0, "")
	return probeResult{
		NodeID:    target.ID,
		Kind:      "quic",
		Success:   true,
		RTT:       rtt,
		IPVersion: ipVersionOf(remoteAddr),
	}
}

func ipVersionOf(addr net.Addr) int {
	var ip net.IP
	switch v := addr.(type) {
	case *net.TCPAddr:
		ip = v.IP
	case *net.UDPAddr:
		ip = v.IP
	}
	if ip == nil {
		return 0
	}
	if ip.To4() != nil {
		return 4
	}
	return 6
}

func shortErr(err error) string {
	if err == nil {
		return ""
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "连接超时"
	}
	s := err.Error()
	if strings.Contains(s, "connection refused") {
		return "端口拒绝连接"
	}
	if strings.Contains(s, "no such host") {
		return "域名解析失败"
	}
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

func logSummary(jobID string, results []probeResult) {
	ok := 0
	for _, r := range results {
		if r.Success {
			ok++
		}
	}
	if jobID != "" {
		log.Printf("[用户 Agent] 任务 %s 完成：%d/%d 可达，已回传面板", jobID, ok, len(results))
	} else {
		log.Printf("[用户 Agent] 实测完成：%d/%d 可达，已回传面板", ok, len(results))
	}
}
