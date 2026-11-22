package xraycore

import (
	"net"
	"testing"
	"time"
)

func TestCloudflareDetection(t *testing.T) {
	cfIPs := []string{"104.16.179.112", "172.67.10.20", "162.159.135.42", "2606:4700:4700::1111"}
	for _, raw := range cfIPs {
		if !IsCloudflareIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s to be detected as Cloudflare IP", raw)
		}
	}
	safeIPs := []string{"23.195.102.161", "23.37.205.51", "142.251.20.91", "151.101.1.91"}
	for _, raw := range safeIPs {
		if IsCloudflareIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s not to be detected as Cloudflare IP", raw)
		}
	}
	if !IsCloudflareTarget("104.16.179.112:443", "example.com", time.Second) {
		t.Fatal("expected Cloudflare IP target to be rejected")
	}
	if !IsCloudflareTarget("www.cloudflare.com:443", "www.cloudflare.com", time.Second) {
		t.Fatal("expected cloudflare.com domain target to be rejected")
	}
	res := CheckRealityTarget("www.cloudflare.com:443", "www.cloudflare.com", time.Second)
	if res.OK || !res.Cloudflare {
		t.Fatalf("expected CheckRealityTarget to reject cloudflare.com, got %#v", res)
	}
}
