package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const agentVersion = "1.1"

type probeNode struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme,omitempty"`
}

type waitEnvelope struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		JobID  string      `json:"jobId"`
		Action string      `json:"action"`
		Nodes  []probeNode `json:"nodes"`
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
		fmt.Println("用户 Agent 已安装并在后台启动，可安全关闭此窗口。")
		return
	}

	if panelURL == "" || tok == "" {
		fmt.Fprintf(os.Stderr, "用法: %s -panel https://your-panel.com -token <TOKEN>\n", os.Args[0])
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: 40 * time.Second}
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

	backoff := 2 * time.Second
	connected := false
	for {
		if ctx.Err() != nil {
			return
		}
		jobID, action, nodes, err := waitForJob(ctx, client, panelURL, tok)
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
			performTracelessUninstall()
			os.Exit(0)
		case "update":
			if err := performSelfUpdate(client, panelURL, tok); err != nil {
				_ = sendAck(ctx, client, panelURL, tok, ackPayload{JobID: jobID, Action: "update", OK: false, Error: err.Error()})
				continue
			}
			_ = sendAck(ctx, client, panelURL, tok, ackPayload{JobID: jobID, Action: "update", OK: true})
			os.Exit(0)
		case "probe", "":
			if len(nodes) == 0 {
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

func waitForJob(ctx context.Context, client *http.Client, panelURL, token string) (string, string, []probeNode, error) {
	url := fmt.Sprintf("%s/api/v1/probe/wait?timeout=25&version=%s", panelURL, agentVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", nil, err
	}
	req.Header.Set("Authorization", "Probe "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", nil, errors.New("令牌无效或已停用")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", "", nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var env waitEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", "", nil, err
	}
	return env.Data.JobID, env.Data.Action, env.Data.Nodes, nil
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
	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return probeResult{NodeID: target.ID, Kind: "tcp", Success: false, RTT: -1, Error: shortErr(err)}
	}
	rtt := max(int(time.Since(start).Milliseconds()), 1)
	ipVer := ipVersionOf(conn.RemoteAddr())
	_ = conn.Close()
	return probeResult{NodeID: target.ID, Kind: "tcp", Success: true, RTT: rtt, IPVersion: ipVer}
}

func buildQUICVersionProbe() []byte {
	pkt := make([]byte, 1200)
	pkt[0] = 0xc0
	pkt[1], pkt[2], pkt[3], pkt[4] = 0x0a, 0x0a, 0x0a, 0x0a
	pkt[5] = 8
	_, _ = rand.Read(pkt[6:14])
	pkt[14] = 8
	_, _ = rand.Read(pkt[15:23])
	return pkt
}

func probeQUIC(ctx context.Context, target probeNode, timeout time.Duration) probeResult {
	addr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp", addr)
	if err != nil {
		return probeResult{NodeID: target.ID, Kind: "quic", Success: false, RTT: -1, Error: shortErr(err)}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	pkt := buildQUICVersionProbe()
	start := time.Now()
	if _, err := conn.Write(pkt); err != nil {
		return probeResult{NodeID: target.ID, Kind: "quic", Success: false, RTT: -1, Error: shortErr(err)}
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil || n < 5 {
		if err == nil {
			err = errors.New("无有效 QUIC 响应")
		}
		return probeResult{NodeID: target.ID, Kind: "quic", Success: false, RTT: -1, Error: shortErr(err)}
	}
	rtt := max(int(time.Since(start).Milliseconds()), 1)
	return probeResult{
		NodeID:    target.ID,
		Kind:      "quic",
		Success:   true,
		RTT:       rtt,
		IPVersion: ipVersionOf(conn.RemoteAddr()),
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
