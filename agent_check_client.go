package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type agentCheckHTTPError int

func (e agentCheckHTTPError) Error() string { return fmt.Sprintf("HTTP %d", int(e)) }

type agentCheckNode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type agentCheckResult struct {
	NodeID    int    `json:"nodeId"`
	Kind      string `json:"kind"`
	Success   bool   `json:"success"`
	RTT       int    `json:"rtt"`
	Error     string `json:"error"`
	IPVersion int    `json:"ipVersion"`
}

type agentCheckJobsResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Nodes           []agentCheckNode `json:"nodes"`
		IntervalSeconds int              `json:"intervalSeconds"`
	} `json:"data"`
}

// runAgentCheckLoop 让每个自建服务器 agent 成为独立的 TCP 检测视角。
func runAgentCheckLoop(ctx context.Context, client *http.Client, panel, token string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		jobs, err := fetchAgentCheckJobs(ctx, client, panel, token)
		if err != nil {
			log.Printf("[agent] 拉取节点检测任务失败: %v", err)
			if !waitContext(ctx, 30*time.Second) {
				return
			}
			continue
		}
		results := checkAgentNodes(ctx, jobs.Data.Nodes, 8, 3*time.Second)
		if len(results) > 0 {
			for attempt := 0; attempt < 3 && ctx.Err() == nil; attempt++ {
				if err := reportAgentChecks(ctx, client, panel, token, results); err == nil {
					break
				} else {
					log.Printf("[agent] 回报节点检测失败: %v", err)
					var status agentCheckHTTPError
					if errors.As(err, &status) && status >= 400 && status < 500 && status != 429 {
						break
					}
					if attempt == 2 {
						break
					}
				}
				if !waitContext(ctx, 30*time.Second) {
					return
				}
			}
		}
		interval := 5 * time.Minute
		if jobs.Data.IntervalSeconds > 0 {
			interval = time.Duration(jobs.Data.IntervalSeconds) * time.Second
		}
		if !waitContext(ctx, interval) {
			return
		}
	}
}

func waitContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func fetchAgentCheckJobs(ctx context.Context, client *http.Client, panel, token string) (*agentCheckJobsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panel+"/api/v1/agent/checks/jobs", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Agent "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, agentCheckHTTPError(resp.StatusCode)
	}
	var out agentCheckJobsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("节点任务失败: %s", out.Msg)
	}
	return &out, nil
}

func checkAgentNodes(ctx context.Context, nodes []agentCheckNode, concurrency int, timeout time.Duration) []agentCheckResult {
	if len(nodes) == 0 || ctx.Err() != nil {
		return nil
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(nodes) {
		concurrency = len(nodes)
	}
	// Bound goroutines as well as concurrent sockets. The old semaphore still
	// allocated one waiting goroutine per node on large subscriptions.
	results := make([]agentCheckResult, len(nodes))
	completed := make([]bool, len(nodes))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				results[i] = checkAgentNode(ctx, nodes[i], timeout)
				completed[i] = true
			}
		}()
	}
dispatch:
	for i := range nodes {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	out := make([]agentCheckResult, 0, len(nodes))
	for i, result := range results {
		if completed[i] {
			out = append(out, result)
		}
	}
	return out
}

func checkAgentNode(ctx context.Context, n agentCheckNode, timeout time.Duration) agentCheckResult {
	result := agentCheckResult{
		NodeID: n.ID, Kind: "tcp", Success: false, RTT: -1,
		IPVersion: probeIPVersion(n.Host),
	}
	start := time.Now()
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", net.JoinHostPort(n.Host, strconv.Itoa(n.Port)))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	_ = conn.Close()
	ms := int(time.Since(start).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	result.Success = true
	result.RTT = ms
	return result
}

func reportAgentChecks(ctx context.Context, client *http.Client, panel, token string, results []agentCheckResult) error {
	body, err := json.Marshal(struct {
		Results []agentCheckResult `json:"results"`
	}{Results: results})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, panel+"/api/v1/agent/checks/report", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Agent "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return agentCheckHTTPError(resp.StatusCode)
	}
	var envelope struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Code != "00000" {
		return fmt.Errorf("节点检测回报失败: %s", envelope.Msg)
	}
	return nil
}
