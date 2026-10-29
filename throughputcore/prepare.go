package throughputcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const toolInstallPath = "/usr/local/lib/subl/iperf3"
const sourceSHA256 = "1c0d0fb02c52626111d6e132db80edfbf27bbaff8bd9245df2a371dcb0b35a92"

// Prepare builds the pinned security-fixed release only on explicit assignment.
// No package manager or host firewall state is changed. Missing build dependencies
// produce a specific failure so an administrator can prepare the host deliberately.
func Prepare(parent context.Context) Capability {
	if c := Inspect(parent); c.AuthSupported {
		return c
	}
	ctx, cancel := context.WithTimeout(parent, 110*time.Second)
	defer cancel()
	fail := func(e error) Capability { c := Inspect(parent); c.Error = e.Error(); return c }
	if runtime.GOOS != "linux" {
		return fail(errors.New("automatic iperf3 preparation is supported on Linux only"))
	}
	for _, name := range []string{"cc", "make", "tar"} {
		if _, e := exec.LookPath(name); e != nil {
			return fail(fmt.Errorf("preparation requires %s and OpenSSL development files; install build dependencies or provision iperf3 >=3.22", name))
		}
	}
	dir, e := os.MkdirTemp("", "subl-iperf-prepare-")
	if e != nil {
		return fail(e)
	}
	defer os.RemoveAll(dir)
	archive := filepath.Join(dir, "source.tgz")
	if e = downloadSource(ctx, archive); e != nil {
		return fail(e)
	}
	commands := []struct {
		name string
		args []string
		cwd  string
	}{{"tar", []string{"xzf", archive, "-C", dir}, dir}, {filepath.Join(dir, "iperf-3.22", "configure"), []string{"--disable-shared", "--with-openssl=/usr"}, filepath.Join(dir, "iperf-3.22")}, {"make", []string{"-j2"}, filepath.Join(dir, "iperf-3.22")}}
	for _, v := range commands {
		cmd := exec.CommandContext(ctx, v.name, v.args...)
		configureCommand(cmd)
		cmd.Dir = v.cwd
		out := &limitedBuffer{max: 4096}
		cmd.Stdout = out
		cmd.Stderr = out
		if e = cmd.Run(); e != nil {
			return fail(fmt.Errorf("iperf3 preparation failed at %s: %v (compiler, make and OpenSSL development files required)", filepath.Base(v.name), e))
		}
	}
	built := filepath.Join(dir, "iperf-3.22", "src", "iperf3")
	c := inspectPath(ctx, built)
	if !c.AuthSupported {
		return fail(errors.New("built iperf3 lacks security-fixed RSA authentication; ensure OpenSSL development files are installed"))
	}
	if e = os.MkdirAll(filepath.Dir(toolInstallPath), 0755); e != nil {
		return fail(e)
	}
	src, e := os.Open(built)
	if e != nil {
		return fail(e)
	}
	defer src.Close()
	dst, e := os.CreateTemp(filepath.Dir(toolInstallPath), "iperf3-new-")
	if e != nil {
		return fail(e)
	}
	name := dst.Name()
	defer os.Remove(name)
	_, e = io.Copy(dst, io.LimitReader(src, 32*1024*1024))
	if e == nil {
		e = dst.Chmod(0755)
	}
	if e == nil {
		e = dst.Sync()
	}
	closeErr := dst.Close()
	if e == nil {
		e = closeErr
	}
	if e == nil {
		e = os.Rename(name, toolInstallPath)
	}
	if e != nil {
		return fail(e)
	}
	return Inspect(parent)
}
func downloadSource(ctx context.Context, path string) error {
	var last error
	for _, url := range []string{"https://downloads.es.net/pub/iperf/iperf-3.22.tar.gz", "https://github.com/esnet/iperf/releases/download/3.22/iperf-3.22.tar.gz"} {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if e != nil {
			return e
		}
		resp, e := (&http.Client{Timeout: 25 * time.Second}).Do(req)
		if e != nil {
			last = e
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			last = fmt.Errorf("source download HTTP %d", resp.StatusCode)
			continue
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if e != nil {
			resp.Body.Close()
			return e
		}
		h := sha256.New()
		n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 2*1024*1024+1))
		resp.Body.Close()
		ce := f.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			last = e
			continue
		}
		if n > 2*1024*1024 || hex.EncodeToString(h.Sum(nil)) != sourceSHA256 {
			return errors.New("iperf3 source SHA256 verification failed")
		}
		return nil
	}
	return fmt.Errorf("could not download pinned iperf3 source: %v", last)
}
