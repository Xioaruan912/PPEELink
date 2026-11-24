package xraycore

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type NetworkTuningState struct {
	BBR     string `json:"bbr"`
	Qdisc   string `json:"qdisc"`
	RmemMax int64  `json:"rmemMax"`
	WmemMax int64  `json:"wmemMax"`
}

func readSysctlString(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readSysctlInt64(path string) int64 {
	s := readSysctlString(path)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// ReadNetworkTuning inspects current Linux congestion control and UDP/TCP buffer sizes.
func ReadNetworkTuning() NetworkTuningState {
	if runtime.GOOS != "linux" {
		return NetworkTuningState{}
	}
	return NetworkTuningState{
		BBR:     readSysctlString("/proc/sys/net/ipv4/tcp_congestion_control"),
		Qdisc:   readSysctlString("/proc/sys/net/core/default_qdisc"),
		RmemMax: readSysctlInt64("/proc/sys/net/core/rmem_max"),
		WmemMax: readSysctlInt64("/proc/sys/net/core/wmem_max"),
	}
}

const ppeelinkSysctlPath = "/etc/sysctl.d/99-ppeelink.conf"

const ppeelinkSysctlContent = `# Managed by PPEELink Agent — BBR + QUIC/Hysteria2 high-throughput buffer tuning
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 262144
net.core.wmem_default = 262144
net.ipv4.udp_rmem_min = 16384
net.ipv4.udp_wmem_min = 16384
net.ipv4.tcp_fastopen = 3
`

// ApplyNetworkTuning persists BBR + fq + 16MB UDP/TCP buffer tuning and applies it live.
func ApplyNetworkTuning() (NetworkTuningState, string, error) {
	if runtime.GOOS != "linux" {
		return NetworkTuningState{}, "", fmt.Errorf("内核网络调优仅支持 Linux 服务器")
	}
	before := ReadNetworkTuning()
	if err := os.MkdirAll("/etc/sysctl.d", 0o755); err != nil {
		return before, "", fmt.Errorf("创建 sysctl.d 失败: %w", err)
	}
	if err := os.WriteFile(ppeelinkSysctlPath, []byte(ppeelinkSysctlContent), 0o644); err != nil {
		return before, "", fmt.Errorf("写入 %s 失败: %w", ppeelinkSysctlPath, err)
	}
	_ = exec.Command("modprobe", "tcp_bbr").Run()
	if out, err := exec.Command("sysctl", "-p", ppeelinkSysctlPath).CombinedOutput(); err != nil {
		return ReadNetworkTuning(), "", fmt.Errorf("sysctl 应用失败: %s", strings.TrimSpace(string(out)))
	}
	after := ReadNetworkTuning()
	summary := fmt.Sprintf("拥塞控制: %s → %s | 队列: %s | UDP 缓冲: %.1fMB → %.1fMB",
		emptyDash(before.BBR), emptyDash(after.BBR),
		emptyDash(after.Qdisc),
		float64(before.RmemMax)/(1024*1024),
		float64(after.RmemMax)/(1024*1024),
	)
	return after, summary, nil
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
