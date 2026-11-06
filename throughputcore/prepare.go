package throughputcore

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

const toolInstallPath = "/usr/local/lib/subl/iperf3"
const staticIPerfVersion = "3.22"
const maxIPerfDownload = 32 * 1024 * 1024

//go:embed licenses/iperf3.txt
var iperfLicense string

//go:embed licenses/openssl.txt
var opensslLicense string

//go:embed licenses/musl.txt
var muslLicense string

type iperfAsset struct{ URL, APIURL, SHA256 string }

// Fixed third-party musl/OpenSSL static release; metadata checked 2026-11-06:
// https://github.com/userdocs/iperf3-static/releases/tag/3.22
func staticIPerfAsset(goos, arch string) (iperfAsset, error) {
	if goos != "linux" {
		return iperfAsset{}, errors.New("自动准备测速工具仅支持 Linux amd64、arm64 和 ARMv7")
	}
	var file, id, sum string
	switch arch {
	case "amd64":
		file = "iperf3-amd64"
		id = "600269270"
		sum = "ef4c89f3fc01c56cf00d34b900024c74f68e540dab452eef4fd902544d9c1c64"
	case "arm64":
		file = "iperf3-arm64v8"
		id = "600269303"
		sum = "34364466812357fe021a8647cb433c8006b0360e3b690ecda470a979a2601031"
	case "armv7":
		file = "iperf3-arm32v7"
		id = "600269285"
		sum = "ebac45408e994c86d78f20dafb1ec7bfd0a5d4cf6acf05283f79c07b2d4647c4"
	default:
		return iperfAsset{}, errors.New("此架构暂不支持自动准备测速工具，请提供支持 RSA 认证的 iperf3 3.22 或更新版本")
	}
	return iperfAsset{URL: "https://github.com/userdocs/iperf3-static/releases/download/" + staticIPerfVersion + "/" + file, APIURL: "https://api.github.com/repos/userdocs/iperf3-static/releases/assets/" + id, SHA256: sum}, nil
}

func runningIPerfArch() string {
	if runtime.GOARCH == "arm" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				if s.Key == "GOARM" && strings.Split(s.Value, ",")[0] == "7" {
					return "armv7"
				}
			}
		}
	}
	return runtime.GOARCH
}

// Prepare installs a verified static program. The target needs no compiler,
// package manager, shared libc or OpenSSL development libraries.
func Prepare(parent context.Context) Capability {
	if c := Inspect(parent); c.AuthSupported {
		return c
	}
	ctx, cancel := context.WithTimeout(parent, 110*time.Second)
	defer cancel()
	fail := func(err error) Capability { c := Inspect(parent); c.Error = err.Error(); return c }
	asset, err := staticIPerfAsset(runtime.GOOS, runningIPerfArch())
	if err != nil {
		return fail(err)
	}
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || req.URL.Scheme != "https" {
			return errors.New("测速工具下载重定向不安全或次数过多")
		}
		return nil
	}}
	if err := prepareStaticIPerf(ctx, client, asset, toolInstallPath); err != nil {
		return fail(err)
	}
	return Inspect(parent)
}

func prepareStaticIPerf(ctx context.Context, client *http.Client, asset iperfAsset, dest string) error {
	first := installStaticIPerf(ctx, client, asset, dest)
	if first == nil || asset.APIURL == "" || ctx.Err() != nil {
		return first
	}
	asset.URL = asset.APIURL + "?ppeelink_download=" + fmt.Sprint(time.Now().UnixNano())
	if err := installStaticIPerf(ctx, client, asset, dest); err != nil {
		return fmt.Errorf("测速工具下载失败：%v；备用下载失败：%w", first, err)
	}
	return nil
}

func installStaticIPerf(ctx context.Context, client *http.Client, asset iperfAsset, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "PPEELink-iperf-Prepare/"+staticIPerfVersion)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载测速工具失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载测速工具 HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxIPerfDownload {
		return errors.New("测速工具文件超过大小限制")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".iperf3-download-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxIPerfDownload+1))
	if err != nil {
		return err
	}
	if n > maxIPerfDownload {
		return errors.New("测速工具文件超过大小限制")
	}
	if hex.EncodeToString(h.Sum(nil)) != asset.SHA256 {
		return errors.New("测速工具 SHA256 校验失败，原工具未更改")
	}
	if err = f.Chmod(0755); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	c := inspectPath(ctx, f.Name())
	if c.Version != staticIPerfVersion || !c.AuthSupported {
		return errors.New("测速工具版本或 RSA 认证校验失败，原工具未更改")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	licenses := "PPEELink iperf3 static distribution (userdocs/iperf3-static 3.22)\n\niperf3\n" + iperfLicense + "\nOpenSSL\n" + opensslLicense + "\nmusl\n" + muslLicense
	if err = os.WriteFile(dest+".LICENSE", []byte(licenses), 0644); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(f.Name(), dest)
}
