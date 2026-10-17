package api

import (
	"strconv"
	"strings"
	"testing"
)

func TestNormalizeSpeedTarget(t *testing.T) {
	out := normalizeSpeedTarget("https://speed.cloudflare.com/__down?bytes=50000000", 10*1024*1024)
	if !strings.Contains(out, "bytes="+strconv.Itoa(10*1024*1024)) {
		t.Fatalf("bytes param not aligned: %s", out)
	}
	if strings.Contains(out, "50000000") {
		t.Fatalf("old bytes value should be replaced: %s", out)
	}
	// 验证 100MB 避免 Cloudflare 404/403
	out100 := normalizeSpeedTarget("https://speed.cloudflare.com/__down?bytes=10000000", 100*1024*1024)
	if !strings.Contains(out100, "bytes=99999999") {
		t.Fatalf("cloudflare 100MB should be capped to 99999999 to avoid 404/403, got %s", out100)
	}
	if got := normalizeSpeedTarget("https://example.com/file.bin", 1024); got != "https://example.com/file.bin" {
		t.Fatalf("url without bytes should be unchanged, got %s", got)
	}
}
