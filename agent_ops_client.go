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
	JobID    string               `json:"jobId"`
	Action   string               `json:"action"` // tune-network | upgrade-core | fetch-logs | check-port | sni-check | security-audit | security-ban | security-unban | unlock-check | set-panel-url | set-policy
	Source   string               `json:"source"`
	Lines    int                  `json:"lines"`
	Filter   string               `json:"filter"`
	Version  string               `json:"version"`
	Port     int                  `json:"port"`
	Proto    string               `json:"proto"`
	Targets  []string             `json:"targets"`
	Ports    []int                `json:"ports"`
	IP       string               `json:"ip"`
	PanelURL string               `json:"panelUrl"`
	Policy   *AgentSecurityPolicy `json:"policy"`
}

type agentOpsResult struct {
	JobID    string                        `json:"jobId"`
	OK       bool                          `json:"ok"`
	Output   string                        `json:"output,omitempty"`
	Error    string                        `json:"error,omitempty"`
	InUse    bool                          `json:"inUse,omitempty"`
	BBR      string                        `json:"bbr,omitempty"`
	Qdisc    string                        `json:"qdisc,omitempty"`
	Rmem     int64                         `json:"rmemMax,omitempty"`
	Wmem     int64                         `json:"wmemMax,omitempty"`
	SNIs     []xraycore.SNICheckResult     `json:"snis,omitempty"`
	Security *xraycore.SecurityAuditResult `json:"security,omitempty"`
	Unlock   *xraycore.ServerUnlockReport  `json:"unlock,omitempty"`
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
		res := executeAgentOpsJob(ctx, runner, token, *job)
		_ = reportAgentOpsResult(ctx, client, panel, token, res)
	}
}

func waitAgentOpsJob(ctx context.Context, client *http.Client, panel, token string) (*agentOpsJob, error) {
	endpoint := strings.TrimRight(panel, "/") + "/api/v1/proxy/agent/ops/wait?timeout=25"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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
	var env struct {
		Data agentOpsJob `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

func executeAgentOpsJob(ctx context.Context, runner *xraycore.Runner, token string, job agentOpsJob) agentOpsResult {
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
	case "sni-check":
		sniCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		res.SNIs = xraycore.CheckRealitySNIs(sniCtx, job.Targets)
		res.OK = true
		res.Output = fmt.Sprintf("已完成 %d 个 Reality 伪装目标体检", len(res.SNIs))
	case "security-audit":
		audit := xraycore.AuditInboundSecurity(runner, job.Ports)
		res.Security = &audit
		res.OK = true
	case "security-ban":
		msg, err := xraycore.BanSourceIP(job.IP)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Output = msg
	case "security-unban":
		msg, err := xraycore.UnbanSourceIP(job.IP)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Output = msg
	case "unlock-check":
		uCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		rep := xraycore.RunServerNativeUnlock(uCtx)
		res.Unlock = &rep
		res.OK = true
		res.Output = "原生解锁检测完成"
	case "set-panel-url":
		msg, err := migrateAgentPanelURL(ctx, job.PanelURL, token)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Output = msg
	case "set-policy":
		if job.Policy == nil {
			res.Error = "策略参数缺失"
			return res
		}
		if err := saveAgentSecurityPolicy(*job.Policy); err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Output = "安全与能力策略已实时生效"
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
	req.Header.Set("Authorization", "Agent "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
