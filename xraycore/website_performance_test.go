package xraycore

import (
	"net"
	"testing"
)

func BenchmarkWebsitePrivateIP(b *testing.B) {
	ip := net.ParseIP("8.8.8.8")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		websitePrivateIP(ip)
	}
}

func TestWebsitePrivateIPRanges(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "192.0.2.1", "198.18.0.1", "203.0.113.1", "::1", "fc00::1", "2001:db8::1", "64:ff9b::808:808", "2002:0808:0808::1"} {
		if !websitePrivateIP(net.ParseIP(raw)) {
			t.Errorf("must deny %s", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if websitePrivateIP(net.ParseIP(raw)) {
			t.Errorf("must allow %s", raw)
		}
	}
}

func TestWebsiteDrainLateWriteRetainsBucket(t *testing.T) {
	counters := newWebsiteCounters()
	key, err := counters.reserve("in-1", "example.com", 3600)
	if err != nil {
		t.Fatal(err)
	}
	counters.add(key, 7, true)
	first := counters.drain()
	counters.add(key, 11, false)
	second := counters.drain()
	if len(first) != 1 || len(second) != 1 || first[0].Up != 7 || second[0].Down != 11 || first[0].BucketStart != second[0].BucketStart {
		t.Fatalf("in-flight write lost or misattributed: %+v %+v", first, second)
	}
}
