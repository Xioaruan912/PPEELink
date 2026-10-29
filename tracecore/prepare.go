package tracecore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const nextTraceInstallPath = "/usr/local/bin/nexttrace"
const maxNextTraceDownload = 32 * 1024 * 1024

type traceAsset struct {
	URL    string
	APIURL string
	SHA256 string
}

// SHA256 digests from GitHub's official v1.7.4 release asset metadata, checked
// 2026-10-29: https://api.github.com/repos/nxtrace/NTrace-core/releases/tags/v1.7.4
func nextTraceAsset(goos, arch string) (traceAsset, error) {
	if goos != "linux" {
		return traceAsset{}, errors.New("自动准备 NextTrace 仅支持 Linux amd64 / arm64")
	}
	sums := map[string]string{"amd64": "25100992ad8924d6bfc7724eb6cc98547d87ec13828aa9b354516687331e170c", "arm64": "84e8dee2115263e3627580382dcb355cf364d5cc4fd4a2a106ac2fd989476834"}
	sum, ok := sums[arch]
	if !ok {
		return traceAsset{}, errors.New("自动准备 NextTrace 仅支持 Linux amd64 / arm64")
	}
	ids := map[string]string{"amd64": "571379707", "arm64": "571379716"}
	return traceAsset{URL: "https://github.com/nxtrace/NTrace-core/releases/download/v" + NextTraceVersion + "/nexttrace_linux_" + arch, APIURL: "https://api.github.com/repos/nxtrace/NTrace-core/releases/assets/" + ids[arch], SHA256: sum}, nil
}

// Prepare runs only for an explicit authenticated assignment. The downloaded
// program is neither installed nor executed before its pinned digest matches.
func Prepare(parent context.Context) error {
	if NextTraceReady() {
		return nil
	}
	asset, err := nextTraceAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 110*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || req.URL.Scheme != "https" {
			return errors.New("不安全或过多的下载重定向")
		}
		return nil
	}}
	if err = prepareNextTrace(ctx, client, asset, nextTraceInstallPath); err != nil {
		return err
	}
	nextTraceCheck.Lock()
	nextTraceCheck.at = time.Time{}
	nextTraceCheck.Unlock()
	return nil
}

func prepareNextTrace(ctx context.Context, client *http.Client, asset traceAsset, dest string) error {
	firstErr := installNextTraceDownload(ctx, client, asset, dest)
	if firstErr == nil || asset.APIURL == "" || ctx.Err() != nil {
		return firstErr
	}
	asset.URL = asset.APIURL + "?subl_download=" + fmt.Sprint(time.Now().UnixNano())
	if err := installNextTraceDownload(ctx, client, asset, dest); err != nil {
		return fmt.Errorf("发布下载失败：%v；官方 API 下载失败：%w", firstErr, err)
	}
	return nil
}

func installNextTraceDownload(ctx context.Context, client *http.Client, asset traceAsset, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "SubL-NextTrace-Prepare/"+NextTraceVersion)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载 NextTrace 失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 NextTrace HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxNextTraceDownload {
		return errors.New("NextTrace 文件超过大小限制")
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".nexttrace-download-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, maxNextTraceDownload+1))
	if err != nil {
		return err
	}
	if n > maxNextTraceDownload {
		return errors.New("NextTrace 文件超过大小限制")
	}
	if hex.EncodeToString(hash.Sum(nil)) != asset.SHA256 {
		return errors.New("NextTrace SHA256 校验失败，原工具未更改")
	}
	if err = f.Chmod(0755); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if !verifyNextTrace(ctx, f.Name()) {
		return errors.New("下载的 NextTrace 版本与固定版本不一致，原工具未更改")
	}
	if err = os.Rename(f.Name(), dest); err != nil {
		return err
	}
	return nil
}
