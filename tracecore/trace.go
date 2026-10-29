// Package tracecore runs bounded, non-shell route diagnostics on panel and probes.
package tracecore

import (
	"context"
	"encoding/json"
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
	if NextTraceReady() {
		return "nexttrace"
	}
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
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	left := MaxOutput - len(b.data)
	if len(p) > left {
		b.truncated = true
	}
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

// Run retains the historical UDP/automatic-family job behavior.
func Run(parent context.Context, raw string) Result { return RunWithOptions(parent, raw, Options{}) }

func RunWithOptions(parent context.Context, raw string, opts Options) Result {
	target, err := ValidateTarget(raw)
	if err != nil {
		return Result{Error: err.Error()}
	}
	opts, err = NormalizeOptions(target, opts)
	if err != nil {
		return Result{Error: err.Error()}
	}
	tool := AvailableTool()
	if tool == "" {
		return Result{Error: "未找到可用追踪工具；NextTrace 需要 v" + NextTraceVersion + "，也可安装 traceroute / tracepath"}
	}
	args, err := CommandArgs(tool, target, opts)
	if err != nil {
		return Result{Tool: tool, Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(parent, Timeout)
	defer cancel()
	executable := tool
	if tool == "nexttrace" {
		executable = nextTraceExecutable()
	}

	if tool == "nexttrace" {
		return runNextTrace(ctx, executable, args)
	}
	return runTraceCommand(ctx, tool, executable, args)
}

// Reserve time for a metadata-free retry if the geolocation service blocks or
// breaks JSON output. The outer 75-second deadline still bounds both attempts.
func runNextTrace(ctx context.Context, executable string, args []string) Result {
	geoCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	result := runTraceCommand(geoCtx, "nexttrace", executable, args)
	cancel()
	if result.Error == "" || ctx.Err() != nil || len(args) == 0 {
		return result
	}
	basicArgs := append([]string{}, args[:len(args)-1]...)
	basicArgs = append(basicArgs, "--data-provider", "disable-geoip", args[len(args)-1])
	return runTraceCommand(ctx, "nexttrace", executable, basicArgs)
}
func runTraceCommand(ctx context.Context, tool, executable string, args []string) Result {
	cmd := exec.CommandContext(ctx, executable, args...)
	out, stderr := &boundedOutput{}, &boundedOutput{}
	cmd.Stdout = out
	cmd.Stderr = stderr
	err := cmd.Run()
	result := Result{Tool: tool, Output: out.String()}
	if ctx.Err() != nil {
		result.Error = "追踪超时或已取消"
	} else if err != nil {
		result.Error = fmt.Sprintf("%s 执行失败：%v %s", tool, err, stderr.String())
	} else if out.truncated {
		result.Error = "追踪输出超过 32 KiB，已截断；请查看原始输出"
	} else if tool == "nexttrace" {
		if _, parseErr := ParseNextTrace(result.Output); parseErr != nil {
			result.Error = "NextTrace JSON 无效：" + parseErr.Error()
		}
	}
	if len(result.Error) > 512 {
		result.Error = strings.ToValidUTF8(result.Error[:509], "") + "…"
	}
	return result
}

type Hop struct {
	TTL       int      `json:"ttl"`
	IP        string   `json:"ip"`
	RTT       *float64 `json:"rtt"`
	Responded bool     `json:"responded"`
	Text      string   `json:"text"`
	Hostname  string   `json:"hostname,omitempty"`
	ASN       string   `json:"asn,omitempty"`
	Country   string   `json:"country,omitempty"`
	Province  string   `json:"province,omitempty"`
	City      string   `json:"city,omitempty"`
	Owner     string   `json:"owner,omitempty"`
	ISP       string   `json:"isp,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

var hopLine = regexp.MustCompile(`^\s*(\d+)(?:\??:)?\s+(.+)$`)
var milliseconds = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*ms`)

func Parse(output string) []Hop {
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		hops, _ := ParseNextTrace(output)
		return hops
	}
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

// ParseNextTrace implements the v1.7.4 normal traceroute JSON contract:
// Result.Hops is [][]Hop; Address is a net.IPAddr object; RTT is nanoseconds.
// https://github.com/nxtrace/NTrace-core/blob/v1.7.4/trace/trace.go
func ParseNextTrace(output string) ([]Hop, error) {
	if len(output) > MaxOutput {
		return nil, errors.New("输出过大")
	}
	var result struct {
		Hops *[][]struct {
			Success bool
			Address *struct {
				IP   string
				Zone string
			}
			Hostname string
			TTL      int
			RTT      int64
			Geo      *struct {
				Asnumber string
				Country  string
				Prov     string
				City     string
				Owner    string
				Isp      string
				Lat      *float64
				Lng      *float64
			}
		}
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, err
	}
	if result.Hops == nil || len(*result.Hops) > MaxHops {
		return nil, errors.New("缺少 Hops 或跳数超限")
	}
	hops := make([]Hop, 0, len(*result.Hops))
	for i, samples := range *result.Hops {
		if len(samples) > 16 {
			return nil, errors.New("跳点样本过多")
		}
		if len(samples) == 0 {
			hops = append(hops, Hop{TTL: i + 1})
			continue
		}
		for _, sample := range samples {
			ttl := sample.TTL
			if ttl == 0 {
				ttl = i + 1
			}
			if ttl != i+1 {
				return nil, errors.New("跳数与结果位置不一致")
			}
			h := Hop{TTL: ttl, Hostname: sample.Hostname}
			if sample.Success {
				if sample.Address == nil || net.ParseIP(sample.Address.IP) == nil || sample.RTT < 0 {
					return nil, errors.New("成功跳点地址或耗时无效")
				}
				h.IP = net.ParseIP(sample.Address.IP).String()
				h.Responded = true
				ms := float64(sample.RTT) / float64(time.Millisecond)
				h.RTT = &ms
			}
			if g := sample.Geo; g != nil {
				h.ASN = g.Asnumber
				h.Country = g.Country
				h.Province = g.Prov
				h.City = g.City
				h.Owner = g.Owner
				h.ISP = g.Isp
				h.Latitude = g.Lat
				h.Longitude = g.Lng
			}
			hops = append(hops, h)
		}
	}
	return hops, nil
}
