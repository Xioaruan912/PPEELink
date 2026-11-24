package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Xioaruan912/PPEELink/xraycore"
)

type agentOpsJob struct {
	JobID   string `json:"jobId"`
	Action  string `json:"action"`  // tune-network | upgrade-core | fetch-logs | check-port
	Source  string `json:"source"`  // xray | agent (for fetch-logs)
	Lines   int    `json:"lines"`   // for fetch-logs
	Filter  string `json:"filter"`  // for fetch-logs
	Version string `json:"version"` // for upgrade-core
	Port    int    `json:"port"`    // for check-port
	Proto   string `json:"proto"`   // tcp | udp (for check-port)
}

type agentOpsResult struct {
	JobID  string `json:"jobId"`
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
	InUse  bool   `json:"inUse,omitempty"`
	BBR    string `json:"bbr,omitempty"`
	Qdisc  string `json:"qdisc,omitempty"`
	Rmem   int64  `json:"rmemMax,omitempty"`
	Wmem   int64  `json:"wmemMax,omitempty"`
}

func runAgentOpsLoop(ctx context.Context, client *http.Client, panel, token string, runner *xraycore.Runner) {
	opsClient := &http.Client{Timeout: 40 * time.Second}
	backoff := 3 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := waitAgentOpsJob(ctx, opsClient, panel, token)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 3 * time.Second
		if job == nil || job.JobID == "" {
			continue
		}
		res := executeAgentOpsJob(ctx, runner, *job)
		_ = reportAgentOpsResult(ctx, client, panel, token, res)
	}
}

func waitAgentOpsJob(ctx context.Context, client *http.Client, panel, token string) (*agentOpsJob, error) {
	endpoint := strings.TrimRight(panel, "/") + "/api/v1/proxy/agent/ops/wait?timeout=25"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var env struct {
		Data agentOpsJob `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

func executeAgentOpsJob(ctx context.Context, runner *xraycore.Runner, job agentOpsJob) agentOpsResult {
	res := agentOpsResult{JobID: job.JobID}
	switch job.Action {
	case "tune-network":
		state, summary, err := xraycore.ApplyNetworkTuning()
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Output = summary
		res.BBR = state.BBR
		res.Qdisc = state.Qdisc
		res.Rmem = state.RmemMax
		res.Wmem = state.WmemMax
	case "upgrade-core":
		upCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		msg, err := xraycore.UpgradeXrayCore(upCtx, runner, job.Version)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Output = msg
	case "fetch-logs":
		res.OK = true
		res.Output = xraycore.FetchServiceLogs(runner, job.Source, job.Lines, job.Filter)
	case "check-port":
		inUse, detail := xraycore.CheckPortInUse(job.Port, job.Proto)
		res.OK = true
		res.InUse = inUse
		res.Output = detail
	default:
		res.Error = "不支持的运维指令: " + job.Action
	}
	return res
}

func reportAgentOpsResult(ctx context.Context, client *http.Client, panel, token string, res agentOpsResult) error {
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(panel, "/") + "/api/v1/proxy/agent/ops/result"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
