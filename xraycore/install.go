package xraycore

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// assetName 官方 release 中对应当前架构的压缩包名。
func assetName(arch string) (string, error) {
	switch arch {
	case "amd64":
		return "Xray-linux-64.zip", nil
	case "arm64":
		return "Xray-linux-arm64-v8a.zip", nil
	default:
		return "", fmt.Errorf("暂不支持的架构: %s", arch)
	}
}

// InstalledVersion 返回 dir 下 xray 的版本号（如 v26.3.27），未安装返回空串。
func InstalledVersion(dir string) string {
	out, err := exec.Command(filepath.Join(dir, "xray"), "version").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 2 && fields[0] == "Xray" {
		return "v" + strings.TrimPrefix(fields[1], "v")
	}
	return ""
}

// EnsureXray 确保 dir 下装有指定版本的 xray：从 GitHub 官方 release 下载，
// 用同一 release 的 .dgst 文件校验 SHA2-256 后再解压，校验失败绝不落盘可执行文件。
func EnsureXray(ctx context.Context, dir, version string) (string, error) {
	if version == "" {
		version = DefaultVersion
	}
	if !versionPattern.MatchString(version) {
		return "", fmt.Errorf("版本号格式无效: %s", version)
	}
	bin := filepath.Join(dir, "xray")
	if InstalledVersion(dir) == version {
		return bin, nil
	}
	asset, err := assetName(runtime.GOARCH)
	if err != nil {
		return "", err
	}
	base := "https://github.com/XTLS/Xray-core/releases/download/" + version + "/" + asset
	client := &http.Client{Timeout: 3 * time.Minute}
	archive, err := download(ctx, client, base, 64<<20)
	if err != nil {
		return "", fmt.Errorf("下载 xray 失败: %w", err)
	}
	digest, err := download(ctx, client, base+".dgst", 4096)
	if err != nil {
		return "", fmt.Errorf("下载校验文件失败: %w", err)
	}
	want := ""
	for _, line := range strings.Split(string(digest), "\n") {
		if strings.HasPrefix(line, "SHA2-256=") {
			want = strings.TrimSpace(strings.TrimPrefix(line, "SHA2-256="))
		}
	}
	sum := sha256.Sum256(archive)
	if want == "" || !strings.EqualFold(hex.EncodeToString(sum[:]), want) {
		return "", fmt.Errorf("xray 压缩包校验失败，已放弃安装")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return "", err
	}
	for _, f := range reader.File {
		name := filepath.Base(f.Name)
		if name != "xray" && name != "geoip.dat" && name != "geosite.dat" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(io.LimitReader(rc, 128<<20))
		rc.Close()
		if err != nil {
			return "", err
		}
		mode := os.FileMode(0o644)
		if name == "xray" {
			mode = 0o755
		}
		// 先写临时文件再改名，避免运行中的进程读到半个文件
		tmp := filepath.Join(dir, "."+name+".new")
		if err := os.WriteFile(tmp, data, mode); err != nil {
			return "", err
		}
		if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
			return "", err
		}
	}
	if got := InstalledVersion(dir); got != version {
		return "", fmt.Errorf("安装后版本不符: %s", got)
	}
	return bin, nil
}

func download(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("文件超过 %d 字节上限", limit)
	}
	return data, nil
}
