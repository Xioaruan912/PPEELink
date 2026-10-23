package xraycore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var executableHashCache struct {
	sync.Mutex
	value string
}

// ExecutableHash 返回当前 agent 可执行文件的 SHA-256，用于判断是否已经是面板新版本。
func ExecutableHash() string {
	executableHashCache.Lock()
	defer executableHashCache.Unlock()
	if executableHashCache.value != "" {
		return executableHashCache.value
	}
	// A running process image is immutable. exec starts a fresh cache; failures
	// remain retryable. Keep hashing /proc/self/exe rather than the replaced path.
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// On Linux the executable path may already point to a newer inode while
	// this process is still running the old one. Hash the running image, not
	// the replacement on disk, so an un-restarted agent cannot confirm upgrade.
	if runtime.GOOS == "linux" {
		exe = "/proc/self/exe"
	}
	file, err := os.Open(exe)
	if err != nil {
		return ""
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return ""
	}
	executableHashCache.value = hex.EncodeToString(sum.Sum(nil))
	return executableHashCache.value
}

// Update 从面板下载当前 agent 二进制，校验 SHA-256 后原子替换并重新执行自身。
// 调用成功后进程通常不会返回；失败时保留旧文件并返回错误。
func (h *HTTPSource) Update(ctx context.Context, expectedHash string) error {
	// API requests use a short timeout, but a binary download may be slow.
	var client *http.Client
	if h.Client != nil {
		copy := *h.Client
		copy.Timeout = 5 * time.Minute
		client = &copy
	}
	return UpdateExecutable(ctx, client, strings.TrimRight(h.Panel, "/")+"/agent/binary?arch="+runtime.GOARCH, "Agent "+h.Token, expectedHash)
}

// UpdateExecutable is shared by the agent and monitor roles of the same program.
func UpdateExecutable(ctx context.Context, client *http.Client, url, authorization, expectedHash string) error {
	if decoded, err := hex.DecodeString(expectedHash); err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("面板未提供有效的 agent 校验值")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 agent 失败：HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxAgentBinaryBytes {
		return fmt.Errorf("agent 下载超过 128 MiB，已保留旧版本")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return replaceExecutable(exe, resp.Body, expectedHash, execSelf)
}

const maxAgentBinaryBytes = 128 << 20

// Stage and hash in one bounded-memory pass. Never rename an incomplete,
// oversized or unverified download over the running executable.
func replaceExecutable(exe string, body io.Reader, expectedHash string, restart func(string) error) error {
	info, err := os.Stat(exe)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(exe), ".agent-update-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	sum := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, sum), io.LimitReader(body, maxAgentBinaryBytes+1))
	if err == nil && n > maxAgentBinaryBytes {
		err = fmt.Errorf("agent 下载超过 128 MiB，已保留旧版本")
	}
	if err == nil && hex.EncodeToString(sum.Sum(nil)) != expectedHash {
		err = fmt.Errorf("agent 校验值不匹配，已保留旧版本")
	}
	if err == nil {
		err = file.Chmod(info.Mode().Perm())
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Keep one rollback inode. A failed exec must not leave the on-disk hash
	// looking upgraded while the old process is still running.
	backup := exe + ".previous"
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Link(exe, backup); err != nil {
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(backup)
		return err
	}
	if err := restart(exe); err != nil {
		if rollbackErr := os.Rename(backup, exe); rollbackErr != nil {
			return fmt.Errorf("重启失败: %v；回滚失败: %w", err, rollbackErr)
		}
		return fmt.Errorf("重启自身失败，已回滚旧版本: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}
