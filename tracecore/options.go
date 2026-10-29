package tracecore

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const NextTraceVersion = "1.7.4"
const ClientVersion = 2

type Options struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Family   string `json:"family"`
}

func NormalizeOptions(target string, o Options) (Options, error) {
	o.Protocol = strings.ToLower(o.Protocol)
	if o.Protocol == "" {
		o.Protocol = "udp"
	}
	if o.Protocol != "icmp" && o.Protocol != "tcp" && o.Protocol != "udp" {
		return o, errors.New("协议必须是 ICMP、TCP 或 UDP")
	}
	if o.Family == "" {
		o.Family = "auto"
	}
	if o.Family != "auto" && o.Family != "4" && o.Family != "6" {
		return o, errors.New("IP 版本必须为 IPv4、IPv6 或自动")
	}
	if o.Port < 0 || o.Port > 65535 {
		return o, errors.New("端口必须在 1–65535 之间")
	}
	if o.Protocol == "icmp" && o.Port != 0 {
		return o, errors.New("ICMP 不使用目标端口")
	}
	if o.Port == 0 {
		if o.Protocol == "tcp" {
			o.Port = 80
		}
		if o.Protocol == "udp" {
			o.Port = 33434
		}
	}
	if ip := net.ParseIP(target); ip != nil {
		if (o.Family == "4" && ip.To4() == nil) || (o.Family == "6" && ip.To4() != nil) {
			return o, errors.New("目标地址与所选 IP 版本不一致")
		}
	}
	return o, nil
}

func LegacyCompatible(o Options) bool {
	return (o.Protocol == "" || o.Protocol == "udp") && (o.Port == 0 || o.Port == 33434) && (o.Family == "" || o.Family == "auto")
}

func CommandArgs(tool, target string, o Options) ([]string, error) {
	var err error
	o, err = NormalizeOptions(target, o)
	if err != nil {
		return nil, err
	}
	family := o.Family
	if family == "auto" {
		if ip := net.ParseIP(target); ip != nil && ip.To4() == nil {
			family = "6"
		}
	}
	switch tool {
	case "nexttrace":
		args := []string{"--json", "--map", "--no-color", "--no-rdns", "--max-hops", strconv.Itoa(MaxHops), "--queries", "1", "--timeout", "1000"}
		if family == "4" {
			args = append(args, "--ipv4")
		}
		if family == "6" {
			args = append(args, "--ipv6")
		}
		if o.Protocol == "tcp" {
			args = append(args, "--tcp")
		}
		if o.Protocol == "udp" {
			args = append(args, "--udp")
		}
		if o.Protocol != "icmp" {
			args = append(args, "--port", strconv.Itoa(o.Port))
		}
		return append(args, target), nil
	case "traceroute":
		args := []string{"-n", "-m", strconv.Itoa(MaxHops), "-w", "1", "-q", "1"}
		if family == "4" || family == "6" {
			args = append(args, "-"+family)
		}
		if o.Protocol == "icmp" {
			args = append(args, "-I")
		}
		if o.Protocol == "tcp" {
			args = append(args, "-T")
		}
		if o.Protocol == "udp" {
			args = append(args, "-U")
		}
		if o.Protocol != "icmp" {
			args = append(args, "-p", strconv.Itoa(o.Port))
		}
		return append(args, target), nil
	case "tracepath":
		if o.Protocol != "udp" {
			return nil, errors.New("tracepath 只支持 UDP；请选择 UDP 或安装 NextTrace / traceroute")
		}
		args := []string{"-n", "-m", strconv.Itoa(MaxHops), "-p", strconv.Itoa(o.Port)}
		if family == "4" || family == "6" {
			args = append(args, "-"+family)
		}
		return append(args, target), nil
	default:
		return nil, errors.New("追踪工具不可用")
	}
}

var nextTraceCheck struct {
	sync.Mutex
	at    time.Time
	ready bool
}
var nextTraceVersionPattern = regexp.MustCompile(`(?i)(?:^|[^0-9.])v?1\.7\.4(?:[^0-9.]|$)`)

// Only a verified contract version is preferred. Other versions leave the
// traditional tools usable and are shown as unavailable for enriched topology.
func NextTraceReady() bool {
	nextTraceCheck.Lock()
	defer nextTraceCheck.Unlock()
	if time.Since(nextTraceCheck.at) < 30*time.Second {
		return nextTraceCheck.ready
	}
	nextTraceCheck.at = time.Now()
	nextTraceCheck.ready = false

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	nextTraceCheck.ready = verifyNextTrace(ctx, nextTraceExecutable())
	return nextTraceCheck.ready
}

func nextTraceExecutable() string {
	if st, err := os.Stat(nextTraceInstallPath); err == nil && st.Mode().IsRegular() && st.Mode()&0111 != 0 {
		return nextTraceInstallPath
	}
	path, _ := exec.LookPath("nexttrace")
	return path
}
func verifyNextTrace(parent context.Context, path string) bool {
	if path == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	out := &boundedOutput{}
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run() == nil && nextTraceVersionPattern.MatchString(out.String())
}
