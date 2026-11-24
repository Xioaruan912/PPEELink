package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// AgentSecurityPolicy governs sensitive host capabilities on this VPS.
// Persisted locally in <dir>/policy.json and updated remotely from the panel.
type AgentSecurityPolicy struct {
	TerminalEnabled bool `json:"terminalEnabled"`
	FilesEnabled    bool `json:"filesEnabled"`
	DockerEnabled   bool `json:"dockerEnabled"`
	AllowAutoUpdate bool `json:"allowAutoUpdate"`
}

var (
	agentPolicyMu   sync.RWMutex
	agentPolicyPath string
	agentPolicy     = AgentSecurityPolicy{
		TerminalEnabled: true,
		FilesEnabled:    true,
		DockerEnabled:   true,
		AllowAutoUpdate: true,
	}
)

func loadAgentSecurityPolicy(dir string) {
	if dir == "" {
		return
	}
	p := filepath.Join(dir, "policy.json")
	agentPolicyMu.Lock()
	agentPolicyPath = p
	if raw, err := os.ReadFile(p); err == nil {
		var parsed AgentSecurityPolicy
		if json.Unmarshal(raw, &parsed) == nil {
			agentPolicy = parsed
		}
	}
	agentPolicyMu.Unlock()
}

func currentAgentPolicy() AgentSecurityPolicy {
	agentPolicyMu.RLock()
	p := agentPolicy
	agentPolicyMu.RUnlock()
	return p
}

func saveAgentSecurityPolicy(p AgentSecurityPolicy) error {
	agentPolicyMu.Lock()
	agentPolicy = p
	path := agentPolicyPath
	agentPolicyMu.Unlock()
	if path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

var systemdPanelArgRe = regexp.MustCompile(`(-panel(?:=|\s+))(["']?)([^\s"']+)(["']?)`)

func rewriteSystemdPanelURL(content, newURL string) (string, bool) {
	if !systemdPanelArgRe.MatchString(content) {
		return "", false
	}
	return systemdPanelArgRe.ReplaceAllString(content, "${1}${2}"+newURL+"${4}"), true
}

// migrateAgentPanelURL pre-flight verifies connectivity to the new panel URL,
// atomically updates the systemd unit file, and schedules a clean non-blocking restart.
func migrateAgentPanelURL(ctx context.Context, newURL, token string) (string, error) {
	uStr := strings.TrimRight(strings.TrimSpace(newURL), "/")
	u, err := url.Parse(uStr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("新面板地址格式无效（需以 http:// 或 https:// 开头）")
	}

	// Pre-flight check: verify the new panel accepts our agent token before rewriting systemd!
	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, uStr+"/api/v1/agent/spec", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Agent "+token)
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("新地址连通性预检失败（已中止切换防止失联）: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("新地址预检返回 HTTP %d（已中止切换防止失联）", resp.StatusCode)
	}

	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("仅支持在 Linux systemd 节点自动迁移面板地址")
	}

	service := "ppeelink-agent"
	if len(os.Args) > 1 && os.Args[1] == "monitor" {
		service = "ppeelink-monitor"
	}
	unitPath := "/etc/systemd/system/" + service + ".service"
	raw, err := os.ReadFile(unitPath)
	if err != nil {
		return "", fmt.Errorf("读取 systemd 配置失败: %w", err)
	}
	content := string(raw)
	updated, ok := rewriteSystemdPanelURL(content, uStr)
	if !ok {
		return "", fmt.Errorf("未在 %s 中找到 -panel 参数", unitPath)
	}
	tmpPath := unitPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("写入新 systemd 配置失败: %w", err)
	}
	if err := os.Rename(tmpPath, unitPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("替换 systemd 配置失败: %w", err)
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return "", fmt.Errorf("systemctl daemon-reload 失败: %s", strings.TrimSpace(string(out)))
	}

	// Delay restart briefly so the HTTP report response reaches the old panel cleanly first.
	go func(svc string) {
		time.Sleep(800 * time.Millisecond)
		_ = exec.Command("systemctl", "restart", "--no-block", svc).Run()
	}(service)

	return fmt.Sprintf("预检通过，已切换面板地址至 %s 并重启服务", uStr), nil
}
