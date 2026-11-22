package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"
)

type agentChinaPingTargetReq struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type agentChinaPingReq struct {
	Targets      []agentChinaPingTargetReq `json:"targets"`
	ZStaticPorts []int                     `json:"zstaticPorts"`
	TimeoutMs    int                       `json:"timeoutMs"`
}

type agentChinaPingTargetResp struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	OK   bool   `json:"ok"`
	Rtt  int    `json:"rtt"`
}

type agentChinaPingZStaticResp struct {
	Port int  `json:"port"`
	OK   bool `json:"ok"`
	Rtt  int  `json:"rtt"`
}

type agentChinaPingResp struct {
	Targets []agentChinaPingTargetResp  `json:"targets"`
	ZStatic []agentChinaPingZStaticResp `json:"zstatic"`
}

func dialDirectTCPStable(ctx context.Context, addr string, timeout time.Duration) int {
	dialOnce := func() int {
		dctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		start := time.Now()
		var d net.Dialer
		conn, err := d.DialContext(dctx, "tcp", addr)
		if err != nil {
			return -1
		}
		_ = conn.Close()
		ms := int(time.Since(start).Milliseconds())
		if ms < 1 {
			ms = 1
		}
		return ms
	}
	first := dialOnce()
	if first < 0 || ctx.Err() != nil {
		return first
	}
	second := dialOnce()
	if second >= 0 && second < first {
		return second
	}
	return first
}

// handleAgentChinaPing executes domestic TCP Ping directly from the Agent's own
// network interface to the target ISP/CDN endpoints, eliminating panel-hop RTT.
func handleAgentChinaPing(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var req agentChinaPingReq
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, errors.New("国内 TCP Ping 请求格式无效")
	}
	if len(req.Targets) > 200 || len(req.ZStaticPorts) > 16 {
		return nil, errors.New("检测目标数量超出上限")
	}
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > 10*time.Second {
		timeout = 3500 * time.Millisecond
	}

	resp := agentChinaPingResp{
		Targets: make([]agentChinaPingTargetResp, len(req.Targets)),
		ZStatic: make([]agentChinaPingZStaticResp, len(req.ZStaticPorts)),
	}

	sem := make(chan struct{}, 24)
	var wg sync.WaitGroup
	for i, t := range req.Targets {
		wg.Add(1)
		go func(idx int, item agentChinaPingTargetReq) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rtt := -1
			if item.IP != "" && item.Port > 0 && item.Port <= 65535 {
				rtt = dialDirectTCPStable(ctx, net.JoinHostPort(item.IP, strconv.Itoa(item.Port)), timeout)
			}
			resp.Targets[idx] = agentChinaPingTargetResp{
				IP: item.IP, Port: item.Port, OK: rtt >= 0, Rtt: rtt,
			}
		}(i, t)
	}
	for i, port := range req.ZStaticPorts {
		wg.Add(1)
		go func(idx, p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rtt := -1
			if p > 0 && p <= 65535 {
				rtt = dialDirectTCPStable(ctx, net.JoinHostPort("s4.zstaticcdn.com", strconv.Itoa(p)), timeout)
			}
			resp.ZStatic[idx] = agentChinaPingZStaticResp{Port: p, OK: rtt >= 0, Rtt: rtt}
		}(i, port)
	}
	wg.Wait()
	return json.Marshal(resp)
}
