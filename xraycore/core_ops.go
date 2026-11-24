package xraycore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// FetchLatestXrayTag queries GitHub Releases for the latest XTLS/Xray-core tag.
func FetchLatestXrayTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/XTLS/Xray-core/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PPEELink-Agent")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("查询最新 Xray 版本失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API 返回 HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil || strings.TrimSpace(rel.TagName) == "" {
		return "", fmt.Errorf("解析最新 Xray 版本失败")
	}
	return strings.TrimSpace(rel.TagName), nil
}

// UpgradeXrayCore downloads and atomically swaps Xray-core with automatic rollback if restart fails.
func UpgradeXrayCore(ctx context.Context, runner *Runner, targetVersion string) (string, error) {
	if targetVersion == "" || strings.EqualFold(targetVersion, "latest") {
		tag, err := FetchLatestXrayTag(ctx)
		if err != nil {
			return "", err
		}
		targetVersion = tag
	}
	tmpDir, err := os.MkdirTemp(runner.Dir, "xray-upgrade-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	newBin, err := EnsureXray(ctx, tmpDir, targetVersion)
	if err != nil {
		return "", fmt.Errorf("下载并校验 Xray %s 失败: %w", targetVersion, err)
	}
	// Verify new binary runs cleanly before swapping.
	if out, err := exec.CommandContext(ctx, newBin, "version").CombinedOutput(); err != nil {
		return "", fmt.Errorf("新版 Xray 校验失败: %s", strings.TrimSpace(string(out)))
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()

	curBin := runner.bin()
	oldBin := curBin + ".old"
	_ = os.Remove(oldBin)
	hadOld := false
	if _, err := os.Stat(curBin); err == nil {
		if err := os.Rename(curBin, oldBin); err != nil {
			return "", fmt.Errorf("备份旧版内核失败: %w", err)
		}
		hadOld = true
	}
	if err := os.Rename(newBin, curBin); err != nil {
		if hadOld {
			_ = os.Rename(oldBin, curBin)
		}
		return "", fmt.Errorf("替换新版内核失败: %w", err)
	}
	_ = os.Chmod(curBin, 0o755)
	runner.versionStamp = ""
	runner.versionCached = ""

	if _, err := os.Stat(runner.configPath()); err == nil {
		if err := runner.restartLocked(); err != nil {
			// Roll back automatically if new core fails to start with active config.
			if hadOld {
				_ = os.Rename(oldBin, curBin)
				_ = runner.restartLocked()
			}
			return "", fmt.Errorf("新版内核启动失败，已自动回滚旧版: %w", err)
		}
	}
	_ = os.Remove(oldBin)
	return fmt.Sprintf("Xray 内核已热升级至 %s", targetVersion), nil
}

// FetchServiceLogs safely reads recent log lines for xray or agent without shell injection.
func FetchServiceLogs(runner *Runner, source string, lines int, filter string) string {
	if lines <= 0 {
		lines = 150
	}
	if lines > 500 {
		lines = 500
	}
	source = strings.ToLower(strings.TrimSpace(source))
	unit := "ppeelink-xray"
	if runner != nil && runner.Unit != "" {
		unit = runner.Unit
	}
	if source == "agent" {
		unit = "ppeelink-agent"
	}

	var raw string
	if runtime.GOOS == "linux" {
		if jctl, err := exec.LookPath("journalctl"); err == nil {
			out, err := exec.Command(jctl, "-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso").CombinedOutput()
			if err == nil && len(out) > 0 {
				raw = string(out)
			}
		}
	}
	if raw == "" && runner != nil && source != "agent" {
		runner.mu.Lock()
		if runner.stderr != nil {
			raw = runner.stderr.String()
		}
		runner.mu.Unlock()
	}
	if raw == "" && runner != nil {
		logFile := filepath.Join(runner.Dir, "error.log")
		if b, err := os.ReadFile(logFile); err == nil {
			raw = string(b)
		}
	}
	if raw == "" {
		return "(暂无日志输出)"
	}
	kw := strings.ToLower(strings.TrimSpace(filter))
	if kw == "" {
		return strings.TrimSpace(raw)
	}
	var matched []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(strings.ToLower(line), kw) {
			matched = append(matched, line)
		}
	}
	if len(matched) == 0 {
		return fmt.Sprintf("(最近 %d 行中没有匹配 %q 的日志)", lines, filter)
	}
	return strings.Join(matched, "\n")
}
