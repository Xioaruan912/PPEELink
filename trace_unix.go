//go:build !windows

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	unixHopLineRe = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	unixIPRe      = regexp.MustCompile(`(\d{1,3}(?:\.\d{1,3}){3}|[0-9a-fA-F:]+:[0-9a-fA-F:]+)`)
	unixRTTRe     = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*ms`)
)

func platformTrace(ctx context.Context, host string) ([]traceHop, error) {
	dstIP, err := resolveTraceTargetIP(host)
	if err != nil {
		return nil, fmt.Errorf("解析目标失败: %w", err)
	}
	wantV6 := dstIP.To4() == nil
	iface := findPhysicalInterface(wantV6)

	bin := "traceroute"
	args := []string{"-n", "-q", "1", "-w", "1", "-m", "24"}
	if wantV6 {
		if _, err := exec.LookPath("traceroute6"); err == nil {
			bin = "traceroute6"
		} else {
			args = append(args, "-6")
		}
	}
	if iface != nil && iface.Name != "" {
		args = append(args, "-i", iface.Name)
	}
	args = append(args, dstIP.String())

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("执行 traceroute 失败: %w", err)
	}

	hops := make([]traceHop, 0, 24)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		m := unixHopLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ttl, _ := strconv.Atoi(m[1])
		body := strings.TrimSpace(m[2])
		if ttl <= 0 {
			continue
		}
		if strings.HasPrefix(body, "*") {
			hops = append(hops, traceHop{TTL: ttl, Timeout: true, RTT: -1})
			continue
		}
		ipMatch := unixIPRe.FindString(body)
		if ipMatch == "" {
			hops = append(hops, traceHop{TTL: ttl, Timeout: true, RTT: -1})
			continue
		}
		rtt := 1
		if rm := unixRTTRe.FindStringSubmatch(body); len(rm) == 2 {
			if f, err := strconv.ParseFloat(rm[1], 64); err == nil {
				rtt = max(int(f+0.5), 1)
			}
		}
		hops = append(hops, traceHop{
			TTL:     ttl,
			IP:      ipMatch,
			RTT:     rtt,
			Timeout: false,
		})
		if ipMatch == dstIP.String() {
			break
		}
	}
	return hops, nil
}
