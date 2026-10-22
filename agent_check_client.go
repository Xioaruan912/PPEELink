package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

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
		jobs, err := fetchAgentCheckJobs(client, panel, token)
		if err != nil {
			log.Printf("[agent] 拉取节点检测任务失败: %v", err)
			if !waitContext(ctx, 30*time.Second) {
				return
			}
			continue
		}
		results := checkAgentNodes(ctx, jobs.Data.Nodes, 8, 3*time.Second)
		if len(results) > 0 {
			if err := reportAgentChecks(client, panel, token, results); err != nil {
				log.Printf("[agent] 回报节点检测失败: %v", err)
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

func fetchAgentCheckJobs(client *http.Client, panel, token string) (*agentCheckJobsResponse, error) {
	req, err := http.NewRequest(http.MethodGet, panel+"/api/v1/agent/checks/jobs", nil)
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
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out agentCheckJobsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func checkAgentNodes(ctx context.Context, nodes []agentCheckNode, concurrency int, timeout time.Duration) []agentCheckResult {
	results := make([]agentCheckResult, len(nodes))
	slots := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Add(1)
		go func(i int, n agentCheckNode) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				return
			}
			results[i] = checkAgentNode(n, timeout)
		}(i, n)
	}
	wg.Wait()
	return results
}

func checkAgentNode(n agentCheckNode, timeout time.Duration) agentCheckResult {
	result := agentCheckResult{
		NodeID: n.ID, Kind: "tcp", Success: false, RTT: -1,
		IPVersion: probeIPVersion(n.Host),
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(n.Host, strconv.Itoa(n.Port)), timeout)
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

func reportAgentChecks(client *http.Client, panel, token string, results []agentCheckResult) error {
	body, err := json.Marshal(struct {
		Results []agentCheckResult `json:"results"`
	}{Results: results})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, panel+"/api/v1/agent/checks/report", bytes.NewReader(body))
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
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
