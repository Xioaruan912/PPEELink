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
	"os/exec"
	"time"

	"ppeelink/tracecore"
)

type agentTraceJob struct {
	ID     uint   `json:"id"`
	Target string `json:"target"`
}

func runAgentTraceLoop(ctx context.Context, client *http.Client, panel, token string) {
	var lastInstall time.Time
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if tracecore.AvailableTool() == "" && (lastInstall.IsZero() || time.Since(lastInstall) > 30*time.Minute) {
			lastInstall = time.Now()
			installTraceTool(ctx)
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

// installTraceTool 尝试用系统包管理器安装 traceroute，避免用户再登录服务器手动操作。
func installTraceTool(parent context.Context) {
	if tracecore.AvailableTool() != "" {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	type attempt struct {
		name string
		args []string
	}
	var attempts []attempt
	switch {
	case lookPath("apt-get") != "":
		attempts = []attempt{
			{"apt-get", []string{"install", "-y", "--no-install-recommends", "traceroute"}},
		}
	case lookPath("dnf") != "":
		attempts = []attempt{{"dnf", []string{"install", "-y", "traceroute"}}}
	case lookPath("yum") != "":
		attempts = []attempt{{"yum", []string{"install", "-y", "traceroute"}}}
	case lookPath("apk") != "":
		attempts = []attempt{{"apk", []string{"add", "--no-cache", "traceroute"}}}
	default:
		log.Printf("[agent] 未找到 traceroute，且系统没有可用的包管理器")
		return
	}
	for _, a := range attempts {
		cmd := exec.CommandContext(ctx, a.name, a.args...)
		cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")
		if out, err := cmd.CombinedOutput(); err == nil {
			log.Printf("[agent] 已自动安装 traceroute")
			return
		} else if len(out) > 0 {
			log.Printf("[agent] 自动安装 traceroute 失败: %v: %s", err, truncateLog(string(out), 300))
		}
	}
	// apt 缓存为空时先 update 再重试一次。
	if lookPath("apt-get") != "" {
		update := exec.CommandContext(ctx, "apt-get", "update")
		_, _ = update.CombinedOutput()
		install := exec.CommandContext(ctx, "apt-get", "install", "-y", "--no-install-recommends", "traceroute")
		install.Env = append(install.Environ(), "DEBIAN_FRONTEND=noninteractive")
		if out, err := install.CombinedOutput(); err == nil {
			log.Printf("[agent] 已自动安装 traceroute")
		} else if len(out) > 0 {
			log.Printf("[agent] 自动安装 traceroute 仍失败: %v: %s", err, truncateLog(string(out), 300))
		}
	}
}

func lookPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func truncateLog(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
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
