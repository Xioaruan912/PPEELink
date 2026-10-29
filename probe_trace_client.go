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

type probeTraceJob struct {
	ID     uint   `json:"id"`
	Target string `json:"target"`
	tracecore.Options
}

// Independent polling keeps interactive traces separate from scheduled TCP checks.
func runProbeTraceLoop(client *http.Client, panel, token string) {
	for {
		jobs, err := fetchProbeTraceJobs(client, panel, token)
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}
		for _, job := range jobs {
			result := tracecore.RunWithOptions(context.Background(), job.Target, job.Options)
			if err := reportProbeTrace(client, panel, token, job.ID, result); err != nil {
				log.Printf("[Probe] 路由追踪回报失败: %v", err)
			}
		}
		time.Sleep(5 * time.Second)
	}
}
func fetchProbeTraceJobs(client *http.Client, panel, token string) ([]probeTraceJob, error) {
	req, err := http.NewRequest("GET", panel+"/api/v1/probe/trace/jobs?version=2&tool="+url.QueryEscape(tracecore.AvailableTool()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Probe "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Data []probeTraceJob `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}
func reportProbeTrace(client *http.Client, panel, token string, id uint, result tracecore.Result) error {
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
	req, err := http.NewRequest("POST", panel+"/api/v1/probe/trace/report", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Probe "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 409 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
