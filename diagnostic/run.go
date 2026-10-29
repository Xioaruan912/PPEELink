package diagnostic

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"ppeelink/xraycore"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Step struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Millis int64  `json:"millis"`
}

// Hop is ordered from the first relay to the final exit; Link never enters reports.
type Hop struct {
	Name string
	Link string
}
type RouteHop struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Transport string `json:"transport"`
}
type Result struct {
	Route       []RouteHop `json:"route,omitempty"`
	Protocol    string     `json:"protocol"`
	Transport   string     `json:"transport"`
	Perspective string     `json:"perspective"`
	Steps       []Step     `json:"steps"`
	Summary     string     `json:"summary"`
	Advice      []string   `json:"advice"`
	CheckedAt   time.Time  `json:"checkedAt"`
}

var slots = make(chan struct{}, 2)
var coreMu sync.Mutex
var plainProbeURL = "http://www.gstatic.com/generate_204"

var targets = map[string]string{"google": "https://www.gstatic.com/generate_204", "cloudflare": "https://www.cloudflare.com/cdn-cgi/trace", "github": "https://github.com/robots.txt"}

type safeLog struct {
	mu   sync.Mutex
	data string
}

func (b *safeLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data += string(p)
	if len(b.data) > 16384 {
		b.data = b.data[len(b.data)-16384:]
	}
	return len(p), nil
}
func (b *safeLog) hint() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := strings.ToLower(b.data)
	switch {
	case strings.Contains(s, "certificate"), strings.Contains(s, "x509"), strings.Contains(s, "certification"):
		return "检测到证书校验相关错误，请核对 SNI、指纹、证书有效期和服务器时间。"
	case strings.Contains(s, "reality verification"):
		return "检测到 Reality 验证失败，请核对公钥、Short ID、SNI 和服务端配置。"
	case strings.Contains(s, "authentication"), strings.Contains(s, "auth failed"):
		return "检测到认证被拒绝，请核对 HY2 密码或账户是否已停用。"
	case strings.Contains(s, "connection refused"):
		return "连接被拒绝，请检查监听端口、服务状态和出口规则。"
	default:
		return "没有足够证据区分握手、账户认证或远端出口故障，请结合服务端日志核对。"
	}
}
func Run(ctx context.Context, link, target string) (*Result, error) {
	return RunPath(ctx, []Hop{{Name: "所选节点", Link: link}}, target)
}
func RunPath(ctx context.Context, path []Hop, target string) (*Result, error) {
	if len(path) == 0 || len(path) > 4 {
		return nil, invalid("诊断路径需包含 1–4 个节点")
	}
	if _, ok := targets[target]; !ok {
		return nil, invalid("检测目标无效")
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return nil, invalid("已有两个诊断正在运行，请稍后重试")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	r := &Result{Perspective: "面板服务器网络", Steps: []Step{}, Advice: []string{}, CheckedAt: time.Now()}
	add := func(key, title, status, detail string, start time.Time) {
		r.Steps = append(r.Steps, Step{key, title, status, detail, time.Since(start).Milliseconds()})
	}

	profiles := []profile{}
	allAddresses := [][]string{}
	// Resolve and pin every hop before making any connection, including later relays.
	for i, hop := range path {
		start := time.Now()
		label := hop.Name
		if label == "" {
			label = fmt.Sprintf("第 %d 跳", i+1)
		}
		p, e := parse(hop.Link)
		if e != nil {
			add(fmt.Sprintf("config-%d", i), label+" · 节点配置", "failed", e.Error(), start)
			r.Summary = "路径节点参数未通过校验"
			return r, nil
		}
		r.Route = append(r.Route, RouteHop{Name: label, Protocol: p.protocol, Transport: p.network})
		add(fmt.Sprintf("config-%d", i), label+" · 节点配置", "passed", "已解析参数，将按保存的中转顺序连接。", start)
		start = time.Now()
		dnsCtx, dc := context.WithTimeout(ctx, 5*time.Second)
		ips, e := net.DefaultResolver.LookupIPAddr(dnsCtx, p.host)
		dc()
		if e != nil || len(ips) == 0 {
			add(fmt.Sprintf("dns-%d", i), label+" · 服务器 DNS", "failed", "无法解析该跳的服务器地址。", start)
			r.Summary = "路径节点域名解析失败"
			return r, nil
		}
		addresses := []string{}
		for _, a := range ips {
			if !publicIP(a.IP.String()) {
				add(fmt.Sprintf("dns-%d", i), label+" · 服务器 DNS", "failed", "该跳包含内网或保留地址，已拒绝整条路径。", start)
				r.Summary = "路径服务器地址不允许用于公网诊断"
				return r, nil
			}
			addresses = append(addresses, a.IP.String())
		}
		add(fmt.Sprintf("dns-%d", i), label+" · 服务器 DNS", "passed", strings.Join(addresses, ", ")+"；本次固定使用解析地址，远端 DNS 可能不同。", start)
		p.pinAddress(addresses[0])
		profiles = append(profiles, p)
		allAddresses = append(allAddresses, addresses)
	}
	first := &profiles[0]
	start := time.Now()
	selected := allAddresses[0][0]
	if first.protocol == "Reality" {
		var e error
		for _, ip := range allAddresses[0] {
			tc, cc := context.WithTimeout(ctx, 3*time.Second)
			conn, err := (&net.Dialer{}).DialContext(tc, "tcp", net.JoinHostPort(ip, strconv.Itoa(first.port)))
			cc()
			e = err
			if e == nil {
				selected = ip
				conn.Close()
				break
			}
		}
		if e != nil {
			add("transport", "入口 TCP 连接", "failed", "入口所有地址均无法连接，尚未进入中转或协议握手。", start)
			r.Summary = "入口服务器端口无法连接"
			r.Advice = []string{"检查入口节点端口、防火墙和服务监听。后续落地节点未被直连测试。"}
			return r, nil
		}
		first.pinAddress(selected)
		add("transport", "入口 TCP 连接", "passed", "入口可连接；后续节点将经中转访问，不会从面板直连。地址："+selected, start)
	} else {
		add("transport", "入口 HY2 UDP 可达性", "unknown", "不使用 TCP 测试判断 UDP；以整条路径的实际 QUIC 请求验证。", start)
	}
	p := profiles[len(profiles)-1]
	p.relays = profiles[:len(profiles)-1]
	r.Protocol = p.protocol
	r.Transport = p.network
	if len(path) > 1 {
		add("route", "中转路径", "passed", "已配置 "+fmt.Sprint(len(path))+" 跳，按入口 → 落地连接；此步骤仅验证配置，通路以下方 HTTPS 为准。", time.Now())
	}
	start = time.Now()
	binary, e := ensureCore(ctx)
	if e != nil {
		add("core", "诊断内核", "failed", "Xray 未准备完成，请稍后重试或检查面板到 GitHub 的下载连接。", start)
		r.Summary = "面板诊断工具尚未就绪"
		return r, nil
	}
	return runCore(ctx, r, p, binary, targets[target], add)
}

var coreReady chan struct{}
var corePath string
var coreError error

func ensureCore(ctx context.Context) (string, error) {
	if xraycore.InstalledVersion("./xray") == xraycore.DefaultVersion {
		return filepath.Abs("./xray/xray")
	}
	coreMu.Lock()
	if coreReady != nil {
		select {
		case <-coreReady:
			if coreError != nil {
				coreReady = nil
			}
		default:
		}
	}
	if coreReady == nil {
		coreReady = make(chan struct{})
		ready := coreReady
		go func() {
			downloadCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			path, err := xraycore.EnsureXray(downloadCtx, "./db/diagnostic-xray", xraycore.DefaultVersion)
			coreMu.Lock()
			corePath, coreError = path, err
			close(ready)
			coreMu.Unlock()
		}()
	}
	ready := coreReady
	coreMu.Unlock()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-ready:
		coreMu.Lock()
		defer coreMu.Unlock()
		return corePath, coreError
	}
}
func runCore(ctx context.Context, r *Result, p profile, binary, target string, add func(string, string, string, string, time.Time)) (*Result, error) {
	start := time.Now()
	dir, e := os.MkdirTemp("", "subl-diagnose-")
	if e != nil {
		return nil, invalid("无法创建诊断临时目录")
	}
	defer os.RemoveAll(dir)
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return nil, invalid("无法分配诊断端口")
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	secret := make([]byte, 24)
	if _, e = rand.Read(secret); e != nil {
		return nil, e
	}
	pass := hex.EncodeToString(secret)
	cfg := filepath.Join(dir, "config.json")
	if e = os.WriteFile(cfg, config(p, port, "diagnostic", pass), 0600); e != nil {
		return nil, invalid("无法写入诊断配置")
	}
	binary, _ = filepath.Abs(binary)
	if e = exec.CommandContext(ctx, binary, "run", "-test", "-c", cfg).Run(); e != nil {
		add("core", "诊断内核配置", "failed", "当前 Xray 拒绝了节点配置，请核对传输、flow、指纹或混淆参数。", start)
		r.Summary = "节点配置与诊断内核不兼容"
		return r, nil
	}
	logs := &safeLog{}
	cmd := exec.CommandContext(ctx, binary, "run", "-c", cfg)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if e = cmd.Start(); e != nil {
		return nil, invalid("无法启动诊断内核")
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	ready := false
	for i := 0; i < 40; i++ {
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if e == nil {
			c.Close()
			ready = true
			break
		}
		select {
		case <-ctx.Done():
			i = 40
		case <-time.After(25 * time.Millisecond):
		}
	}
	if !ready {
		add("core", "诊断内核配置", "failed", "独立诊断进程未能启动。", start)
		r.Summary = "面板诊断进程启动失败"
		return r, nil
	}
	add("core", "诊断内核配置", "passed", "已启动隔离的 Xray 客户端，不会修改节点配置。", start)
	proxyURL, _ := url.Parse(fmt.Sprintf("socks5://diagnostic:%s@127.0.0.1:%d", pass, port))
	tr := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true, TLSHandshakeTimeout: 6 * time.Second, ResponseHeaderTimeout: 8 * time.Second}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	start = time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", plainProbeURL, nil)
	resp, e := client.Do(req)
	tunnelOK := e == nil
	if tunnelOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		add("tunnel", "协议握手、认证与出口", "passed", fmt.Sprintf("已通过节点收到 HTTP %d，确认该请求的握手、认证和出口转发可用。", resp.StatusCode), start)
	} else {
		add("tunnel", "协议握手、认证与出口", "unknown", logs.hint(), start)
	}
	start = time.Now()
	var tlsOK atomic.Bool
	req, _ = http.NewRequestWithContext(ctx, "GET", target, nil)
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{TLSHandshakeDone: func(_ tls.ConnectionState, e error) {
		if e == nil {
			tlsOK.Store(true)
		}
	}}))
	resp, e = client.Do(req)
	if e == nil {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		add("https", "目标网站 HTTPS", "passed", fmt.Sprintf("已通过节点访问所选目标，HTTP %d。HTTP 403/429 等表示网站已响应，可能限制当前出口。", resp.StatusCode), start)
		r.Summary = "代理可连接，所选网站已响应"
		if len(p.relays) > 0 {
			r.Summary = "中转通路可连接，所选网站已响应"
		}
		if resp.StatusCode >= 400 {
			r.Advice = append(r.Advice, "节点通路已建立，目标网站返回限制或错误，请更换出口或检查网站策略。")
		}
		if !tunnelOK {
			r.Advice = append(r.Advice, "HTTPS 已成功，前面的 HTTP 探测失败可能是远端只允许 HTTPS，不能据此判定节点失效。")
		}
	} else {
		status := "unknown"
		detail := logs.hint()
		if tlsOK.Load() {
			status = "failed"
			detail = "已完成目标网站 TLS 握手，但没有收到完整 HTTP 响应，检查网站限制或远端链路。"
		} else if tunnelOK {
			detail = "代理出口已验证，但所选网站的 DNS、TLS 或访问策略仍可能存在问题。"
		}
		add("https", "目标网站 HTTPS", status, detail, start)
		r.Summary = "尚未定位到唯一故障环节"
		if len(p.relays) > 0 {
			detail += " 当前包含中转，无法仅凭整链超时断定具体哪一跳认证失败；请分别检测入口和落地，并核对入口 UDP 转发。"
		}
		r.Advice = []string{detail, "Reality 请核对 UUID、公钥、Short ID、SNI、flow 与 XHTTP 路径。HY2 请核对 UDP 放行、密码、证书指纹和混淆密码。", "诊断从面板网络出发；若面板可用而手机不可用，请检查手机网络、客户端配置和本地 DNS。"}
	}
	return r, nil
}
