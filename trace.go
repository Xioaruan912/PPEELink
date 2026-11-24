package main

import (
	"context"
	"net"
	"strings"
)

type traceHop struct {
	TTL     int    `json:"ttl"`
	IP      string `json:"ip"`
	RTT     int    `json:"rtt"`
	Timeout bool   `json:"timeout"`
}

type traceReportPayload struct {
	JobID string     `json:"jobId"`
	Host  string     `json:"host"`
	Hops  []traceHop `json:"hops"`
	Error string     `json:"error,omitempty"`
}

func resolveTraceTargetIP(host string) (net.IP, error) {
	clean := strings.Trim(strings.TrimSpace(host), "[]")
	if ip := net.ParseIP(clean); ip != nil {
		return ip, nil
	}
	addrs, err := net.LookupIP(clean)
	if err != nil || len(addrs) == 0 {
		return nil, err
	}
	for _, a := range addrs {
		if v4 := a.To4(); v4 != nil {
			return v4, nil
		}
	}
	return addrs[0], nil
}

func runOutboundTrace(ctx context.Context, host string) ([]traceHop, error) {
	return platformTrace(ctx, host)
}
