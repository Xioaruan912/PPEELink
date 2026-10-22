package xraycore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"
)

// ExecutableHash 返回当前 agent 可执行文件的 SHA-256，用于判断是否已经是面板新版本。
func ExecutableHash() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
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
	return hex.EncodeToString(sum.Sum(nil))
}

// Update 从面板下载当前 agent 二进制，校验 SHA-256 后原子替换并重新执行自身。
// 调用成功后进程通常不会返回；失败时保留旧文件并返回错误。
func (h *HTTPSource) Update(ctx context.Context, expectedHash string) error {
	if expectedHash == "" {
		return fmt.Errorf("面板未提供 agent 校验值")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		h.Panel+"/api/v1/agent/binary?arch="+runtime.GOARCH, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Agent "+h.Token)
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 agent 失败：HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != expectedHash {
		return fmt.Errorf("agent 校验值不匹配，已保留旧版本")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	info, err := os.Stat(exe)
	if err != nil {
		return err
	}
	tmp := exe + ".update"
	if err := os.WriteFile(tmp, body, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Chmod(tmp, info.Mode().Perm()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := execSelf(exe); err != nil {
		return fmt.Errorf("替换成功但重启自身失败：%w", err)
	}
	return nil
}
