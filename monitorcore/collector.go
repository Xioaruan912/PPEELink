package monitorcore

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Metric 是一次 Linux 服务器指标采样。
type Metric struct {
	CPUPercent   float64 `json:"cpuPercent"`
	MemUsed      uint64  `json:"memUsed"`
	MemTotal     uint64  `json:"memTotal"`
	SwapUsed     uint64  `json:"swapUsed"`
	SwapTotal    uint64  `json:"swapTotal"`
	DiskUsed     uint64  `json:"diskUsed"`
	DiskTotal    uint64  `json:"diskTotal"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
	Uptime       uint64  `json:"uptime"`
	NetInSpeed   uint64  `json:"netInSpeed"`
	NetOutSpeed  uint64  `json:"netOutSpeed"`
	TransferIn   uint64  `json:"transferIn"`
	TransferOut  uint64  `json:"transferOut"`
	TCPConns     int     `json:"tcpConns"`
	UDPConns     int     `json:"udpConns"`
	ProcessCount int     `json:"processCount"`
}

type cpuStat struct {
	total uint64
	idle  uint64
}

type netStat struct {
	in  uint64
	out uint64
}

// Collector 保存上一次 CPU / 网络计数，用于计算增量。
type Collector struct {
	prevCPU  cpuStat
	prevNet  netStat
	prevTime time.Time
}

func NewCollector() *Collector { return &Collector{} }

func (c *Collector) Collect() Metric {
	now := time.Now()
	metrics := Metric{}
	metrics.CPUPercent = c.cpuPercent()
	readMemory(&metrics)
	readDisk(&metrics)
	readLoad(&metrics)
	readUptime(&metrics)
	c.readNetwork(&metrics, now)
	readProcessCount(&metrics)
	metrics.TCPConns = countProcNet("/proc/net/tcp") + countProcNet("/proc/net/tcp6")
	metrics.UDPConns = countProcNet("/proc/net/udp") + countProcNet("/proc/net/udp6")
	return metrics
}

func (c *Collector) cpuPercent() float64 {
	stat, err := readCPUStat()
	if err != nil {
		return 0
	}
	prev := c.prevCPU
	c.prevCPU = stat
	if prev.total == 0 || stat.total <= prev.total {
		return 0
	}
	total := stat.total - prev.total
	idle := stat.idle - prev.idle
	if idle > total {
		idle = total
	}
	if total == 0 {
		return 0
	}
	return float64(total-idle) * 100 / float64(total)
}

func readCPUStat() (cpuStat, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStat{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		var total, idle uint64
		for i, raw := range fields {
			v, _ := strconv.ParseUint(raw, 10, 64)
			total += v
			if i == 3 || i == 4 { // idle + iowait
				idle += v
			}
		}
		return cpuStat{total: total, idle: idle}, nil
	}
	return cpuStat{}, fmt.Errorf("no cpu line")
}

func readMemory(m *Metric) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	values := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		values[key] = v * 1024
	}
	m.MemTotal = values["MemTotal"]
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"]
	}
	if m.MemTotal > available {
		m.MemUsed = m.MemTotal - available
	}
	m.SwapTotal = values["SwapTotal"]
	if m.SwapTotal > values["SwapFree"] {
		m.SwapUsed = m.SwapTotal - values["SwapFree"]
	}
}

func readDisk(m *Metric) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return
	}
	m.DiskTotal = stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if m.DiskTotal > free {
		m.DiskUsed = m.DiskTotal - free
	}
}

func readLoad(m *Metric) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		m.Load1, _ = strconv.ParseFloat(fields[0], 64)
		m.Load5, _ = strconv.ParseFloat(fields[1], 64)
		m.Load15, _ = strconv.ParseFloat(fields[2], 64)
	}
}

func readUptime(m *Metric) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		seconds, _ := strconv.ParseFloat(fields[0], 64)
		m.Uptime = uint64(seconds)
	}
}

func (c *Collector) readNetwork(m *Metric, now time.Time) {
	stat, err := readNetStat()
	if err != nil {
		return
	}
	m.TransferIn = stat.in
	m.TransferOut = stat.out
	if c.prevTime.IsZero() {
		c.prevNet = stat
		c.prevTime = now
		return
	}
	seconds := now.Sub(c.prevTime).Seconds()
	if seconds < 1 {
		c.prevNet = stat
		c.prevTime = now
		return
	}
	if stat.in >= c.prevNet.in {
		m.NetInSpeed = uint64(float64(stat.in-c.prevNet.in) / seconds)
	}
	if stat.out >= c.prevNet.out {
		m.NetOutSpeed = uint64(float64(stat.out-c.prevNet.out) / seconds)
	}
	c.prevNet = stat
	c.prevTime = now
}

func readNetStat() (netStat, error) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return netStat{}, err
	}
	var total netStat
	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		if name == "lo" || name == "" {
			continue
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 9 {
			continue
		}
		in, _ := strconv.ParseUint(fields[0], 10, 64)
		out, _ := strconv.ParseUint(fields[8], 10, 64)
		total.in += in
		total.out += out
	}
	return total, nil
}

func readProcessCount(m *Metric) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) < 4 {
		return
	}
	parts := strings.Split(fields[3], "/")
	if len(parts) == 2 {
		m.ProcessCount, _ = strconv.Atoi(parts[1])
	}
}

func countProcNet(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	if count > 0 {
		count--
	}
	return count
}
