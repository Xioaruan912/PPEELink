// Package tracecore runs bounded, non-shell route diagnostics on panel and probes.
package tracecore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MaxOutput = 32 * 1024
const MaxHops = 24
const Timeout = 75 * time.Second

var label = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

func ValidateTarget(raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if len(target) == 0 || len(target) > 253 {
		return "", errors.New("请输入域名或 IP，不要包含协议、路径或端口")
	}
	if ip := net.ParseIP(target); ip != nil {
		if ip.IsUnspecified() || ip.IsMulticast() {
			return "", errors.New("不支持未指定或组播地址")
		}
		return ip.String(), nil
	}
	target = strings.TrimSuffix(target, ".")
	for _, part := range strings.Split(target, ".") {
		if !label.MatchString(part) {
			return "", errors.New("目标必须是合法域名或 IP；不接受 URL、空格和命令参数")
		}
	}
	return strings.ToLower(target), nil
}

func AvailableTool() string {
	for _, name := range []string{"traceroute", "tracepath"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

type Result struct {
	Tool   string `json:"tool"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

type boundedOutput struct {
	mu   sync.Mutex
	data []byte
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	left := MaxOutput - len(b.data)
	if left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func (b *boundedOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.ToValidUTF8(string(b.data), "�")
}

func Run(parent context.Context, raw string) Result {
	target, err := ValidateTarget(raw)
	if err != nil {
		return Result{Error: err.Error()}
	}
	tool := AvailableTool()
	if tool == "" {
		return Result{Error: "未安装 traceroute 或 tracepath；请在该来源服务器安装 traceroute 后重试"}
	}
	ctx, cancel := context.WithTimeout(parent, Timeout)
	defer cancel()
	args := []string{"-n", "-m", strconv.Itoa(MaxHops), "-w", "1", "-q", "1", target}
	if tool == "tracepath" {
		args = []string{"-n", "-m", strconv.Itoa(MaxHops), target}
	} else if ip := net.ParseIP(target); ip != nil && ip.To4() == nil {
		args = append([]string{"-6"}, args...)
	}
	cmd := exec.CommandContext(ctx, tool, args...)
	out := &boundedOutput{}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	result := Result{Tool: tool, Output: out.String()}
	if ctx.Err() != nil {
		result.Error = "追踪超时或已取消"
	} else if err != nil {
		result.Error = fmt.Sprintf("%s 执行失败：%v", tool, err)
	}
	return result
}

type Hop struct {
	TTL       int      `json:"ttl"`
	IP        string   `json:"ip"`
	RTT       *float64 `json:"rtt"`
	Responded bool     `json:"responded"`
	Text      string   `json:"text"`
}

var hopLine = regexp.MustCompile(`^\s*(\d+)(?:\??:)?\s+(.+)$`)
var milliseconds = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*ms`)

func Parse(output string) []Hop {
	hops := make([]Hop, 0)
	for _, line := range strings.Split(output, "\n") {
		match := hopLine.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		ttl, _ := strconv.Atoi(match[1])
		if ttl < 1 || ttl > MaxHops {
			continue
		}
		h := Hop{TTL: ttl, Text: strings.TrimSpace(match[2])}
		for _, field := range strings.Fields(match[2]) {
			field = strings.Trim(field, "()[]")
			if ip := net.ParseIP(field); ip != nil {
				h.IP = ip.String()
				h.Responded = true
				break
			}
		}
		if m := milliseconds.FindStringSubmatch(match[2]); len(m) == 2 {
			v, _ := strconv.ParseFloat(m[1], 64)
			h.RTT = &v
		}
		hops = append(hops, h)
	}
	return hops
}
