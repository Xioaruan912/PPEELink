package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
)

type netDiagCheck struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
	Advice string `json:"advice,omitempty"`
	RTTMs  int    `json:"rttMs,omitempty"`
}

type netDiagReport struct {
	Summary   string         `json:"summary"`
	CheckedAt string         `json:"checkedAt"`
	Checks    []netDiagCheck `json:"checks"`
}

// runNetDiag 执行 3 秒级「代理接管 & DNS & TUN」一键网络体检。
func runNetDiag(ctx context.Context) *netDiagReport {
	checks := make([]netDiagCheck, 5)
	st := detectClashStatus(ctx)
	_, sec := clashCandidates()
	var rtCfg clashRuntimeConfig
	if st.Connected && st.Controller != "" {
		rtCfg = getClashRuntimeConfig(ctx, st.Controller, sec)
	}

	var wg sync.WaitGroup
	wg.Add(5)

	// 1. Clash 控制器与本地代理端口
	go func() {
		defer wg.Done()
		if !st.Connected {
			checks[0] = netDiagCheck{
				Key:    "clash_api",
				Label:  "Clash 外部控制接口",
				Status: "fail",
				Detail: "未检测到 Clash / Mihomo 控制器（9090 / 9097）",
				Advice: "请确认 Clash 客户端已启动并开启 External Controller",
			}
			return
		}
		port := rtCfg.MixedPort
		if port == 0 {
			port = rtCfg.Port
		}
		if port == 0 {
			port = 7890
		}
		mode := rtCfg.Mode
		if mode == "" {
			mode = "rule"
		}
		checks[0] = netDiagCheck{
			Key:    "clash_api",
			Label:  "Clash 外部控制接口",
			Status: "ok",
			Detail: fmt.Sprintf("%s · 模式 %s · 本地端口 %d · 当前节点 %s", st.Controller, mode, port, st.Current),
		}
	}()

	// 2. DNS 与 Fake-IP 接管检测
	go func() {
		defer wg.Done()
		t0 := time.Now()
		dCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		defer cancel()
		gIPs, gErr := net.DefaultResolver.LookupIP(dCtx, "ip4", "www.google.com")
		bIPs, bErr := net.DefaultResolver.LookupIP(dCtx, "ip4", "www.baidu.com")
		rtt := int(time.Since(t0).Milliseconds())

		isFakeIP := func(ips []net.IP) bool {
			for _, ip := range ips {
				if v4 := ip.To4(); v4 != nil && v4[0] == 198 && (v4[1] == 18 || v4[1] == 19) {
					return true
				}
			}
			return false
		}
		gFake := isFakeIP(gIPs)
		bFake := isFakeIP(bIPs)
		switch {
		case gErr != nil && bErr != nil:
			checks[1] = netDiagCheck{
				Key:    "dns_fakeip",
				Label:  "系统 DNS 与 Fake-IP",
				Status: "fail",
				Detail: "系统 DNS 解析失败",
				Advice: "检查 Clash DNS 监听或重启 TUN 虚拟网卡",
				RTTMs:  rtt,
			}
		case gFake && !bFake:
			checks[1] = netDiagCheck{
				Key:    "dns_fakeip",
				Label:  "系统 DNS 与 Fake-IP",
				Status: "ok",
				Detail: "国外域名已命中 Fake-IP（198.18.x.x），国内域名直连真实 IP",
				RTTMs:  rtt,
			}
		case gFake && bFake:
			checks[1] = netDiagCheck{
				Key:    "dns_fakeip",
				Label:  "系统 DNS 与 Fake-IP",
				Status: "warn",
				Detail: "国内与国外域名均命中 Fake-IP（TUN 全接管模式）",
				Advice: "建议在订阅中确认国内直连域名已加入 fake-ip-filter",
				RTTMs:  rtt,
			}
		default:
			ipStr := "未知"
			if len(gIPs) > 0 {
				ipStr = gIPs[0].String()
			}
			checks[1] = netDiagCheck{
				Key:    "dns_fakeip",
				Label:  "系统 DNS 与 Fake-IP",
				Status: "warn",
				Detail: fmt.Sprintf("未检测到 Fake-IP（google 解析为 %s）", ipStr),
				Advice: "如开启了 TUN 模式仍打不开海外站，请开启 Clash DNS 接管并刷新系统 DNS 缓存",
				RTTMs:  rtt,
			}
		}
	}()

	// 3. 国内直连连通性
	go func() {
		defer wg.Done()
		t0 := time.Now()
		c := &http.Client{
			Timeout:   3 * time.Second,
			Transport: &http.Transport{Proxy: nil},
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://www.baidu.com", nil)
		resp, err := c.Do(req)
		rtt := int(time.Since(t0).Milliseconds())
		if err != nil {
			checks[2] = netDiagCheck{
				Key:    "cn_direct",
				Label:  "国内直连通道",
				Status: "fail",
				Detail: "直连百度失败: " + shortErr(err),
				Advice: "检查本地网络或是否误将国内站点走代理",
				RTTMs:  rtt,
			}
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		status := "ok"
		advice := ""
		if rtt > 350 {
			status = "warn"
			advice = "国内直连延迟偏高，可能被 TUN 误转发至海外代理"
		}
		checks[2] = netDiagCheck{
			Key:    "cn_direct",
			Label:  "国内直连通道",
			Status: status,
			Detail: fmt.Sprintf("直连 www.baidu.com HTTP %d（%d ms）", resp.StatusCode, rtt),
			Advice: advice,
			RTTMs:  rtt,
		}
	}()

	// 4. 代理出口实测（经 Clash 本地 mixed-port 访问 Cloudflare Trace）
	go func() {
		defer wg.Done()
		if !st.Connected {
			checks[3] = netDiagCheck{
				Key:    "proxy_exit",
				Label:  "海外代理出口实测",
				Status: "fail",
				Detail: "未连接 Clash，跳过代理出口检测",
				Advice: "启动 Clash 后重新体检",
			}
			return
		}
		t0 := time.Now()
		client := newClashBatchHTTPClient(st.Controller, rtCfg, 4*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.cloudflare.com/cdn-cgi/trace", nil)
		resp, err := client.Do(req)
		rtt := int(time.Since(t0).Milliseconds())
		if err != nil {
			checks[3] = netDiagCheck{
				Key:    "proxy_exit",
				Label:  "海外代理出口实测",
				Status: "fail",
				Detail: "经代理访问海外失败: " + shortErr(err),
				Advice: "当前节点可能不可用，请在面板切换节点后重试",
				RTTMs:  rtt,
			}
			return
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		var ip, loc, colo string
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "ip=") {
				ip = strings.TrimSpace(strings.TrimPrefix(line, "ip="))
			} else if strings.HasPrefix(line, "loc=") {
				loc = strings.TrimSpace(strings.TrimPrefix(line, "loc="))
			} else if strings.HasPrefix(line, "colo=") {
				colo = strings.TrimSpace(strings.TrimPrefix(line, "colo="))
			}
		}
		if strings.EqualFold(loc, "CN") {
			checks[3] = netDiagCheck{
				Key:    "proxy_exit",
				Label:  "海外代理出口实测",
				Status: "warn",
				Detail: fmt.Sprintf("出口落在 CN（%s · %s），海外流量未正确接管", ip, colo),
				Advice: "请确认 Clash 处于 Rule/Global 模式且策略组未选中 DIRECT",
				RTTMs:  rtt,
			}
			return
		}
		checks[3] = netDiagCheck{
			Key:    "proxy_exit",
			Label:  "海外代理出口实测",
			Status: "ok",
			Detail: fmt.Sprintf("出口 %s · 地区 %s · 机房 %s（%d ms）", ip, loc, colo, rtt),
			RTTMs:  rtt,
		}
	}()

	// 5. UDP / QUIC 连通性
	go func() {
		defer wg.Done()
		t0 := time.Now()
		qCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		defer cancel()
		conn, err := quic.DialAddr(qCtx, "1.1.1.1:443", &tls.Config{
			ServerName: "cloudflare.com",
			NextProtos: []string{"h3"},
		}, &quic.Config{HandshakeIdleTimeout: 2 * time.Second})
		rtt := int(time.Since(t0).Milliseconds())
		if err != nil {
			checks[4] = netDiagCheck{
				Key:    "udp_quic",
				Label:  "UDP / QUIC 通道",
				Status: "warn",
				Detail: "QUIC/UDP 直连受限或被运营商 QoS",
				Advice: "若使用 Hysteria2 / TUIC 出现丢包，建议优先切换 VLESS Reality TCP 节点",
				RTTMs:  rtt,
			}
			return
		}
		_ = conn.CloseWithError(0, "diag")
		checks[4] = netDiagCheck{
			Key:    "udp_quic",
			Label:  "UDP / QUIC 通道",
			Status: "ok",
			Detail: fmt.Sprintf("HTTP/3 QUIC 握手正常（%d ms）", rtt),
			RTTMs:  rtt,
		}
	}()

	wg.Wait()

	okCount, warnCount, failCount := 0, 0, 0
	for _, c := range checks {
		switch c.Status {
		case "ok":
			okCount++
		case "warn":
			warnCount++
		default:
			failCount++
		}
	}
	summary := fmt.Sprintf("体检完成：%d 项正常", okCount)
	if warnCount > 0 {
		summary += fmt.Sprintf("，%d 项提醒", warnCount)
	}
	if failCount > 0 {
		summary += fmt.Sprintf("，%d 项异常", failCount)
	}
	return &netDiagReport{
		Summary:   summary,
		CheckedAt: time.Now().Format(time.RFC3339),
		Checks:    checks,
	}
}
