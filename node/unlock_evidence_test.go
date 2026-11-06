package node

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	singbox "github.com/sagernet/sing-box"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type unlockTransport func(*http.Request) (*http.Response, error)

func (f unlockTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func unlockClient(code int, body string) *http.Client {
	return &http.Client{Transport: unlockTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	})}
}
func unlockService(t *testing.T, key string) UnlockService {
	t.Helper()
	for _, s := range ListUnlockServices() {
		if s.Key == key {
			return s
		}
	}
	t.Fatalf("missing service %s", key)
	return UnlockService{}
}
func TestUnlockEvidenceClassification(t *testing.T) {
	for _, tc := range []struct {
		name, key              string
		code                   int
		body, status, evidence string
		ok                     bool
	}{
		{"openai explicit response", "openai", 200, `{"required":false}`, "available", "api", true},
		{"openai country refusal", "openai", 403, `{"error":{"code":"unsupported_country_region_territory"}}`, "blocked", "api", false},
		{"openai rate limited", "openai", 429, `{"error":{"message":"try later"}}`, "unknown", "api", false},
		{"openai unrelated JSON", "openai", 200, `{"error":{"message":"unauthorized"}}`, "unknown", "api", false},
		{"openai challenge", "openai", 403, `<html>Just a moment...</html>`, "unknown", "api", false},
		{"own service region", "claude", 200, "ip=203.0.113.1\nloc=JP\n", "inferred", "region", false},
		{"restricted own region", "claude", 200, "loc=CN\n", "region_blocked", "region", false},
		{"challenge with fake region", "claude", 403, "loc=JP\n", "unknown", "region", false},
		{"trace not trace", "claude", 200, "<html>Sign in</html>", "unknown", "region", false},
		{"youtube only reachable", "youtube", 204, "", "reachable", "reach", false},
		{"youtube HTTP failure", "youtube", 503, "error", "unknown", "reach", false},
		{"dazn refused", "dazn", 200, `{"isAllowed":false,"GeolocatedCountry":"JP"}`, "blocked", "api", false},
		{"dazn allowed", "dazn", 200, `{"isAllowed":true,"GeolocatedCountry":"US"}`, "available", "api", true},
		{"tvb allowed", "tvb", 200, `{"allow_in_this_country":true,"country":"SG"}`, "available", "api", true},
		{"gemini missing fragile marker", "google-gemini", 200, "<html>Login</html>", "reachable", "reach", false},
		{"disney homepage", "disney", 200, "<html>Disney+</html>", "reachable", "reach", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := probeUnlock(context.Background(), unlockClient(tc.code, tc.body), unlockService(t, tc.key))
			if r.Status != tc.status || r.Evidence != tc.evidence || r.Ok != tc.ok {
				t.Fatalf("got %+v; want %s/%s ok=%v", r, tc.status, tc.evidence, tc.ok)
			}
		})
	}
}
func TestUnlockNetflixNetworkErrorIsUnknown(t *testing.T) {
	c := &http.Client{Transport: unlockTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	r := probeUnlock(context.Background(), c, unlockService(t, "netflix"))
	if r.Ok || r.Status != "unknown" {
		t.Fatalf("network error must not unlock: %+v", r)
	}
}
func TestUnlockCatalogAndFilters(t *testing.T) {
	ss := ListUnlockServices()
	if len(ss) != 18 {
		t.Fatalf("got %d services", len(ss))
	}
	for _, s := range ss {
		if !strings.HasPrefix(s.URL, "https://") || s.Key == "" || s.Name == "" || s.Group == "" {
			t.Fatalf("invalid catalog entry %+v", s)
		}
	}
	if _, e := SelectUnlockServices("missing", ""); e == nil {
		t.Fatal("unknown service accepted")
	}
	if _, e := SelectUnlockServices("", "missing"); e == nil {
		t.Fatal("unknown group accepted")
	}
	selected, e := SelectUnlockServices("", "ai")
	if e != nil || len(selected) != 8 {
		t.Fatalf("ai filter %d, %v", len(selected), e)
	}
	RegisterUnlockService(UnlockService{Key: "z-test", Name: "Custom", Group: "forum", Check: func(context.Context, *http.Client) UnlockCheckResult { return UnlockCheckResult{Status: "unknown"} }})
	defer func() { unlockRegistry.Lock(); delete(unlockRegistry.services, "z-test"); unlockRegistry.Unlock() }()
	if ListUnlockServices()[len(ss)].Key != "z-test" {
		t.Fatal("newly registered service omitted")
	}
}
func TestUnlockRunnerBoundsConcurrencyAndCancellation(t *testing.T) {
	var active, peak atomic.Int32
	c := &http.Client{Transport: unlockTransport(func(r *http.Request) (*http.Response, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	ss := make([]UnlockService, 12)
	for i := range ss {
		ss[i] = UnlockService{Key: "probe", Name: "Probe", Group: "forum", Kind: "reach", URL: "https://example.test/"}
	}
	r := runUnlockChecks(ctx, c, ss, time.Second, nil)
	if peak.Load() > 4 || peak.Load() == 0 {
		t.Fatalf("peak=%d", peak.Load())
	}
	if len(r) != 12 {
		t.Fatal("lost results")
	}
	for _, v := range r {
		if v.Key == "" || v.Status != "unknown" || v.Ok {
			t.Fatalf("cancelled result %+v", v)
		}
	}
}

func TestUnlockGeneratedProxyConfigParses(t *testing.T) {
	outbound := `{"type":"shadowsocks","tag":"proxy","server":"192.0.2.1","server_port":443,"method":"aes-128-gcm","password":"fixture"}`
	config := buildSingboxConfig(1080, outbound)
	if _, err := parseOptions(config); err != nil {
		t.Fatalf("generated proxy config is invalid: %v", err)
	}
}

func TestUnlockRunUsesSelectedProxyAndStreamsResults(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer target.Close()
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`{"log":{"disabled":true},"inbounds":[{"type":"shadowsocks","listen":"127.0.0.1","listen_port":%d,"method":"aes-128-gcm","password":"fixture"}],"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`, port)
	options, err := parseOptions(config)
	if err != nil {
		t.Fatal(err)
	}
	server, err := singbox.New(singbox.Options{Context: getBoxContext(), Options: *options})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		server.Close()
		t.Fatal(err)
	}
	defer server.Close()
	key := "local-proxy-fixture"
	RegisterUnlockService(UnlockService{Key: key, Name: "Fixture", Group: "forum", Kind: "reach", URL: target.URL})
	defer func() { unlockRegistry.Lock(); delete(unlockRegistry.services, key); unlockRegistry.Unlock() }()
	link := fmt.Sprintf("ss://%s@127.0.0.1:%d#Fixture", base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:fixture")), port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	received := 0
	result, err := RunUnlockTest(ctx, UnlockTestConfig{Link: link, ServiceFilter: key, OnResult: func(r UnlockCheckResult) {
		received++
		if r.Status != "reachable" {
			t.Errorf("unexpected stream row %+v", r)
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Complete || result.Source != "node" || received != 1 || len(result.Results) != 1 || result.Results[0].Status != "reachable" {
		t.Fatalf("invalid proxy-run result %+v, events=%d", result, received)
	}
}
