package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"ppeelink/tracecore"
)

type agentTraceJob struct {
	ID     uint   `json:"id"`
	Target string `json:"target"`
}

func runAgentTraceLoop(ctx context.Context, client *http.Client, panel, token string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		jobs, err := fetchAgentTraceJobs(client, panel, token)
		if err == nil {
			for _, job := range jobs {
				result := tracecore.Run(ctx, job.Target)
				if reportErr := reportAgentTrace(client, panel, token, job.ID, result); reportErr != nil {
					log.Printf("[agent] 路由追踪回报失败: %v", reportErr)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func fetchAgentTraceJobs(client *http.Client, panel, token string) ([]agentTraceJob, error) {
	req, err := http.NewRequest(http.MethodGet, panel+"/api/v1/agent/trace/jobs?tool="+url.QueryEscape(tracecore.AvailableTool()), nil)
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
	var payload struct {
		Data []agentTraceJob `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func reportAgentTrace(client *http.Client, panel, token string, id uint, result tracecore.Result) error {
	if len(result.Error) > 512 {
		result.Error = result.Error[:512]
	}
	body, err := json.Marshal(struct {
		ID     uint   `json:"id"`
		Tool   string `json:"tool"`
		Output string `json:"output"`
		Error  string `json:"error"`
	}{id, result.Tool, result.Output, result.Error})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, panel+"/api/v1/agent/trace/report", bytes.NewReader(body))
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
