package xraycore

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const banChainName = "PPEELINK_BAN"

type ClientConnStat struct {
	IP    string `json:"ip"`
	Conns int    `json:"conns"`
	Ports []int  `json:"ports"`
}

type SecurityAlertIP struct {
	IP         string `json:"ip"`
	Hits       int    `json:"hits"`
	LastReason string `json:"lastReason"`
	Banned     bool   `json:"banned"`
}

type SecurityAuditResult struct {
	ActiveConns     int               `json:"activeConns"`
	UniqueClientIPs int               `json:"uniqueClientIps"`
	Clients         []ClientConnStat  `json:"clients"`
	Alerts          []SecurityAlertIP `json:"alerts"`
	BannedIPs       []string          `json:"bannedIps"`
}

var (
	logIPv4Re = regexp.MustCompile(`\b((?:\d{1,3}\.){3}\d{1,3})(?::\d+)?\b`)
)

func isPublicClientIP(raw string) bool {
	ip := net.ParseIP(strings.Trim(strings.TrimSpace(raw), "[]"))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return true
}

func parseHostPortLoose(addr string) (string, int) {
	addr = strings.TrimSpace(addr)
	if h, p, err := net.SplitHostPort(addr); err == nil {
		port, _ := strconv.Atoi(p)
		return strings.Trim(h, "[]"), port
	}
	if idx := strings.LastIndexByte(addr, ':'); idx > 0 {
		port, _ := strconv.Atoi(addr[idx+1:])
		return strings.Trim(addr[:idx], "[]"), port
	}
	return "", 0
}

// AuditInboundSecurity inspects active connections on inbound ports, aggregates
// recent failed handshakes / scan probes from Xray logs, and lists banned IPs.
func AuditInboundSecurity(runner *Runner, inboundPorts []int) SecurityAuditResult {
	res := SecurityAuditResult{
		Clients:   []ClientConnStat{},
		Alerts:    []SecurityAlertIP{},
		BannedIPs: ListBannedIPs(),
	}
	bannedSet := make(map[string]bool, len(res.BannedIPs))
	for _, ip := range res.BannedIPs {
		bannedSet[ip] = true
	}

	portSet := make(map[int]bool, len(inboundPorts))
	for _, p := range inboundPorts {
		if p > 0 {
			portSet[p] = true
		}
	}

	// 1. Active connections per client IP on Linux via `ss -Hntu state established`.
	if runtime.GOOS == "linux" && len(portSet) > 0 {
		if ssBin, err := exec.LookPath("ss"); err == nil {
			out, err := exec.Command(ssBin, "-Hntu", "state", "established").CombinedOutput()
			if err == nil {
				byIP := map[string]*ClientConnStat{}
				ipPorts := map[string]map[int]bool{}
				for _, line := range strings.Split(string(out), "\n") {
					fields := strings.Fields(line)
					// ss -Hntu columns: Netid Recv-Q Send-Q Local Peer
					if len(fields) < 5 {
						continue
					}
					_, localPort := parseHostPortLoose(fields[3])
					if !portSet[localPort] {
						continue
					}
					remoteIP, _ := parseHostPortLoose(fields[4])
					remoteIP = strings.TrimPrefix(remoteIP, "::ffff:")
					if !isPublicClientIP(remoteIP) {
						continue
					}
					res.ActiveConns++
					st, ok := byIP[remoteIP]
					if !ok {
						st = &ClientConnStat{IP: remoteIP}
						byIP[remoteIP] = st
						ipPorts[remoteIP] = map[int]bool{}
					}
					st.Conns++
					ipPorts[remoteIP][localPort] = true
				}
				for ip, st := range byIP {
					for p := range ipPorts[ip] {
						st.Ports = append(st.Ports, p)
					}
					sort.Ints(st.Ports)
					res.Clients = append(res.Clients, *st)
				}
				sort.Slice(res.Clients, func(i, j int) bool {
					if res.Clients[i].Conns != res.Clients[j].Conns {
						return res.Clients[i].Conns > res.Clients[j].Conns
					}
					return res.Clients[i].IP < res.Clients[j].IP
				})
				if len(res.Clients) > 25 {
					res.Clients = res.Clients[:25]
				}
				res.UniqueClientIPs = len(byIP)
			}
		}
	}

	// 2. Aggregate suspicious probe / handshake failure IPs from recent Xray logs.
	rawLogs := FetchServiceLogs(runner, "xray", 400, "")
	type agg struct {
		hits   int
		reason string
	}
	suspects := map[string]*agg{}
	for _, line := range strings.Split(rawLogs, "\n") {
		low := strings.ToLower(line)
		reason := ""
		switch {
		case strings.Contains(low, "invalid user") || strings.Contains(low, "authentication failed"):
			reason = "非法凭据 / 认证失败"
		case strings.Contains(low, "tls: first record") || strings.Contains(low, "not a tls handshake") || strings.Contains(low, "bad record mac"):
			reason = "非 TLS 端口扫描"
		case strings.Contains(low, "rejected") && (strings.Contains(low, "proxy/vless") || strings.Contains(low, "proxy/vmess") || strings.Contains(low, "reality")):
			reason = "协议探测被拒绝"
		case strings.Contains(low, "failed to process response") || strings.Contains(low, "invalid request"):
			reason = "畸形请求探测"
		}
		if reason == "" {
			continue
		}
		matches := logIPv4Re.FindAllStringSubmatch(line, 2)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			ip := m[1]
			if !isPublicClientIP(ip) {
				continue
			}
			a, ok := suspects[ip]
			if !ok {
				a = &agg{}
				suspects[ip] = a
			}
			a.hits++
			a.reason = reason
			break
		}
	}
	for ip, a := range suspects {
		res.Alerts = append(res.Alerts, SecurityAlertIP{
			IP:         ip,
			Hits:       a.hits,
			LastReason: a.reason,
			Banned:     bannedSet[ip],
		})
	}
	sort.Slice(res.Alerts, func(i, j int) bool {
		if res.Alerts[i].Hits != res.Alerts[j].Hits {
			return res.Alerts[i].Hits > res.Alerts[j].Hits
		}
		return res.Alerts[i].IP < res.Alerts[j].IP
	})
	if len(res.Alerts) > 15 {
		res.Alerts = res.Alerts[:15]
	}
	return res
}

func ensureBanChain(ipt string) error {
	_ = exec.Command(ipt, "-N", banChainName).Run()
	if err := exec.Command(ipt, "-C", "INPUT", "-j", banChainName).Run(); err != nil {
		if out, ierr := exec.Command(ipt, "-I", "INPUT", "1", "-j", banChainName).CombinedOutput(); ierr != nil {
			return fmt.Errorf("挂载封禁链失败: %s", strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// ListBannedIPs lists IPv4 addresses currently dropped in the PPEELINK_BAN iptables chain.
func ListBannedIPs() []string {
	if runtime.GOOS != "linux" {
		return []string{}
	}
	ipt, err := exec.LookPath("iptables")
	if err != nil {
		return []string{}
	}
	out, err := exec.Command(ipt, "-S", banChainName).CombinedOutput()
	if err != nil {
		return []string{}
	}
	ips := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "-s" {
				ip := strings.TrimSuffix(fields[i+1], "/32")
				if net.ParseIP(ip) != nil {
					ips = append(ips, ip)
				}
			}
		}
	}
	return ips
}

// BanSourceIP adds an iptables DROP rule for a validated public IPv4 source.
func BanSourceIP(rawIP string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil || ip.To4() == nil || !isPublicClientIP(ip.String()) {
		return "", fmt.Errorf("仅支持封禁合法的公网 IPv4 地址")
	}
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("仅支持在 Linux 服务器上执行封禁")
	}
	ipt, err := exec.LookPath("iptables")
	if err != nil {
		return "", fmt.Errorf("未找到 iptables")
	}
	if err := ensureBanChain(ipt); err != nil {
		return "", err
	}
	target := ip.String() + "/32"
	if exec.Command(ipt, "-C", banChainName, "-s", target, "-j", "DROP").Run() == nil {
		return fmt.Sprintf("%s 已在封禁列表中", ip.String()), nil
	}
	if out, err := exec.Command(ipt, "-A", banChainName, "-s", target, "-j", "DROP").CombinedOutput(); err != nil {
		return "", fmt.Errorf("添加封禁规则失败: %s", strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("已封禁来源 IP %s", ip.String()), nil
}

// UnbanSourceIP removes an IPv4 DROP rule from PPEELINK_BAN.
func UnbanSourceIP(rawIP string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("IP 地址格式无效")
	}
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("仅支持在 Linux 服务器上执行解封")
	}
	ipt, err := exec.LookPath("iptables")
	if err != nil {
		return "", fmt.Errorf("未找到 iptables")
	}
	target := ip.String() + "/32"
	for exec.Command(ipt, "-C", banChainName, "-s", target, "-j", "DROP").Run() == nil {
		_ = exec.Command(ipt, "-D", banChainName, "-s", target, "-j", "DROP").Run()
	}
	return fmt.Sprintf("已解封来源 IP %s", ip.String()), nil
}
