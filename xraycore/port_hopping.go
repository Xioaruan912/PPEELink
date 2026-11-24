package xraycore

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	hopCommentTag = "ppeelink-hop"
	fwCommentTag  = "ppeelink-fw"
)

// ParsePortRange parses a port hopping range string like "30000-40000".
func ParsePortRange(raw string) (int, int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, false
	}
	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || start < 1024 || end > 65535 || start >= end {
		return 0, 0, false
	}
	return start, end, true
}

// SyncPortHopping configures idempotent iptables/ip6tables NAT PREROUTING
// UDP port-hopping REDIRECT rules for Hysteria2 inbounds on Linux.
func SyncPortHopping(inbounds []InboundSpec) {
	if runtime.GOOS != "linux" {
		return
	}
	ClearPortHopping()
	for _, in := range inbounds {
		if in.Template != TemplateHysteria2 || in.Port <= 0 {
			continue
		}
		start, end, ok := ParsePortRange(in.PortHopping)
		if !ok {
			continue
		}
		dport := fmt.Sprintf("%d:%d", start, end)
		toPort := strconv.Itoa(in.Port)
		if bin, err := exec.LookPath("iptables"); err == nil {
			_ = exec.Command(bin, "-t", "nat", "-A", "PREROUTING",
				"-p", "udp", "--dport", dport,
				"-m", "comment", "--comment", hopCommentTag,
				"-j", "REDIRECT", "--to-ports", toPort,
			).Run()
		}
		if bin6, err := exec.LookPath("ip6tables"); err == nil {
			_ = exec.Command(bin6, "-t", "nat", "-A", "PREROUTING",
				"-p", "udp", "--dport", dport,
				"-m", "comment", "--comment", hopCommentTag,
				"-j", "REDIRECT", "--to-ports", toPort,
			).Run()
		}
	}
}

// ClearPortHopping removes all ppeelink-hop NAT PREROUTING rules cleanly.
func ClearPortHopping() {
	if runtime.GOOS != "linux" {
		return
	}
	for _, tool := range []string{"iptables", "ip6tables"} {
		bin, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		out, err := exec.Command(bin, "-t", "nat", "-S", "PREROUTING").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, hopCommentTag) {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "-A" {
				fields[0] = "-D"
				args := append([]string{"-t", "nat"}, fields...)
				_ = exec.Command(bin, args...).Run()
			}
		}
	}
}

// SyncAutoFirewall opens required TCP/UDP ports (and port-hopping ranges)
// when AutoFirewall is enabled on the server, tagging rules for clean removal.
func SyncAutoFirewall(enabled bool, inbounds []InboundSpec) {
	if runtime.GOOS != "linux" {
		return
	}
	ClearAutoFirewall()
	if !enabled || len(inbounds) == 0 {
		return
	}
	bin, err := exec.LookPath("iptables")
	if err != nil {
		return
	}
	bin6, _ := exec.LookPath("ip6tables")

	addRule := func(proto, dport string) {
		_ = exec.Command(bin, "-I", "INPUT", "1",
			"-p", proto, "--dport", dport,
			"-m", "comment", "--comment", fwCommentTag,
			"-j", "ACCEPT",
		).Run()
		if bin6 != "" {
			_ = exec.Command(bin6, "-I", "INPUT", "1",
				"-p", proto, "--dport", dport,
				"-m", "comment", "--comment", fwCommentTag,
				"-j", "ACCEPT",
			).Run()
		}
	}

	seen := map[string]bool{}
	for _, in := range inbounds {
		if in.Port <= 0 {
			continue
		}
		pStr := strconv.Itoa(in.Port)
		if in.Template == TemplateHysteria2 {
			if !seen["udp:"+pStr] {
				seen["udp:"+pStr] = true
				addRule("udp", pStr)
			}
			if start, end, ok := ParsePortRange(in.PortHopping); ok {
				rng := fmt.Sprintf("%d:%d", start, end)
				if !seen["udp:"+rng] {
					seen["udp:"+rng] = true
					addRule("udp", rng)
				}
			}
		} else {
			if !seen["tcp:"+pStr] {
				seen["tcp:"+pStr] = true
				addRule("tcp", pStr)
			}
			if in.Template == TemplateSS2022 && !seen["udp:"+pStr] {
				seen["udp:"+pStr] = true
				addRule("udp", pStr)
			}
		}
	}
}

// ClearAutoFirewall removes all ppeelink-fw INPUT rules cleanly.
func ClearAutoFirewall() {
	if runtime.GOOS != "linux" {
		return
	}
	for _, tool := range []string{"iptables", "ip6tables"} {
		bin, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		out, err := exec.Command(bin, "-S", "INPUT").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, fwCommentTag) {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "-A" {
				fields[0] = "-D"
				_ = exec.Command(bin, fields...).Run()
			}
		}
	}
}

// CheckPortInUse tests whether a local TCP or UDP port is currently occupied.
func CheckPortInUse(port int, proto string) (bool, string) {
	if port <= 0 || port > 65535 {
		return true, "端口超出有效范围"
	}
	addr := fmt.Sprintf(":%d", port)
	if strings.EqualFold(proto, "udp") {
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			return true, fmt.Sprintf("UDP %d 已被占用", port)
		}
		_ = pc.Close()
		return false, fmt.Sprintf("UDP %d 可用", port)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true, fmt.Sprintf("TCP %d 已被占用", port)
	}
	_ = ln.Close()
	return false, fmt.Sprintf("TCP %d 可用", port)
}
