package xraycore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func egressResolveSpec(t *testing.T) Spec {
	t.Helper()
	s := websiteTestSpec()
	s.WebsiteTraffic = nil
	s.Inbounds = s.Inbounds[:1]
	s.Inbounds[0].Egress = hy2TestEgress(t, `,"alpn":["h3"],"pinSHA256":"`+strings.Repeat("ab", 32)+`"`)
	return s
}

func egressAnswerIPs(values ...string) []netip.Addr {
	ips := make([]netip.Addr, len(values))
	for i, value := range values {
		ips[i] = netip.MustParseAddr(value)
	}
	return ips
}

func TestPrepareEgressRejectsEveryNonPublicAnswer(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "::ffff:127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.169.254", "::", "fe80::1", "fd00::1", "224.0.0.1", "0.1.2.3", "2001:db8::1", "2001::1", "64:ff9b::7f00:1"} {
		for _, mixed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/mixed-%t", address, mixed), func(t *testing.T) {
				s := egressResolveSpec(t)
				rev := s.Rev()
				answers := egressAnswerIPs(address)
				if mixed {
					answers = append(egressAnswerIPs("1.1.1.1"), answers...)
				}
				_, err := prepareEgress(context.Background(), s, func(context.Context, string) ([]netip.Addr, error) { return answers, nil })
				if err == nil || !strings.Contains(err.Error(), "in-7") || !strings.Contains(err.Error(), "exit.example.com") || !strings.Contains(err.Error(), "非公网") {
					t.Fatalf("unsafe answer not rejected with an actionable error: %v", err)
				}
				if s.Rev() != rev || s.Inbounds[0].Egress.Address != "exit.example.com" {
					t.Fatal("failed preparation mutated the desired configuration")
				}
			})
		}
	}
	for _, answers := range [][]netip.Addr{nil, {{}}, {netip.MustParseAddr("2606:4700:4700::1111%eth0")}, make([]netip.Addr, egressMaxAnswers+1)} {
		if _, err := prepareEgress(context.Background(), egressResolveSpec(t), func(context.Context, string) ([]netip.Addr, error) { return answers, nil }); err == nil {
			t.Fatal("empty, malformed, scoped or oversized DNS result accepted")
		}
	}
}

func TestPrepareEgressPinsAliasesWithoutChangingTLSOrDesiredSpec(t *testing.T) {
	s := egressResolveSpec(t)
	in := &s.Inbounds[0]
	in.Egress.Address = "EXIT.Example.com."
	in.Users = []UserSpec{{Email: "stable-user", UUID: "11111111-1111-4111-8111-111111111111"}}
	reality := &EgressSpec{Address: "exit.example.com", Port: 24444, UUID: in.Users[0].UUID, SNI: "reality.example.com", PublicKey: strings.Repeat("A", 43), Transport: "xhttp", Path: "/stable"}
	in.DomainRoutes = []DomainRouteSpec{{Domain: "example.org", Match: "suffix", Egress: reality}, {Domain: "direct.example.net", Match: "exact"}, {Domain: "example.net", Match: "suffix", Egress: in.Egress}}
	before, _ := json.Marshal(s)
	rev := s.Rev()
	queries := []string{}
	prepared, err := prepareEgress(context.Background(), s, func(_ context.Context, host string) ([]netip.Addr, error) {
		queries = append(queries, host)
		return egressAnswerIPs("2606:4700:4700::1111", "8.8.8.8", "::ffff:1.1.1.1", "1.1.1.1"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(queries, []string{"exit.example.com"}) {
		t.Fatalf("case/trailing-dot aliases were resolved repeatedly: %v", queries)
	}
	if prepared.Inbounds[0].Egress.Address != "1.1.1.1" || prepared.Inbounds[0].DomainRoutes[0].Egress.Address != "1.1.1.1" || prepared.Inbounds[0].DomainRoutes[2].Egress.Address != "1.1.1.1" {
		t.Fatal("default and rule exits were not pinned to the same validated address")
	}
	for _, metered := range []bool{false, true} {
		candidate := prepared
		var collector *WebsiteCollector
		if metered {
			candidate.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
			collector = websiteTestCollector(t, candidate)
		}
		config, err := BuildConfigWithWebsiteCollector(candidate, collector)
		if err != nil {
			t.Fatal(err)
		}
		var cfg map[string]any
		if err := json.Unmarshal(config, &cfg); err != nil {
			t.Fatal(err)
		}
		seen := 0
		for _, value := range cfg["outbounds"].([]any) {
			out := value.(map[string]any)
			if out["protocol"] != "hysteria" && out["protocol"] != "vless" {
				continue
			}
			stream := out["streamSettings"].(map[string]any)
			settings := out["settings"].(map[string]any)
			if out["protocol"] == "hysteria" {
				tls := stream["tlsSettings"].(map[string]any)
				if settings["address"] != "1.1.1.1" || tls["serverName"] != "exit.example.com" || tls["pinnedPeerCertSha256"] != strings.Repeat("ab", 32) || stream["hysteriaSettings"].(map[string]any)["auth"] != "test-secret" {
					t.Fatalf("HY2 pinned endpoint lost TLS/auth settings: %v", out)
				}
			} else {
				server := settings["vnext"].([]any)[0].(map[string]any)
				if server["address"] != "1.1.1.1" || stream["realitySettings"].(map[string]any)["serverName"] != "reality.example.com" || stream["xhttpSettings"].(map[string]any)["path"] != "/stable" {
					t.Fatalf("Reality pinned endpoint lost transport/SNI settings: %v", out)
				}
			}
			seen++
		}
		if seen != 3 {
			t.Fatalf("expected all three physical outbounds, got %d", seen)
		}
	}
	// None of the nested slices/pointers used during runtime preparation may
	// alias the panel's desired spec, including shared EgressSpec pointers.
	prepared.Inbounds[0].Egress.ALPN[0] = "changed"
	prepared.Inbounds[0].DomainRoutes[0].Egress.SNI = "changed"
	prepared.Inbounds[0].Users[0].UUID = "changed"
	after, _ := json.Marshal(s)
	if !bytes.Equal(before, after) || s.Rev() != rev {
		t.Fatal("preparation changed the desired revision or nested credentials")
	}
}

func TestPrepareEgressLiteralAndRebinding(t *testing.T) {
	s := egressResolveSpec(t)
	calls := 0
	lookup := func(context.Context, string) ([]netip.Addr, error) {
		calls++
		if calls > 1 {
			return egressAnswerIPs("127.0.0.1"), nil
		}
		return egressAnswerIPs("8.8.8.8"), nil
	}
	pinned, err := prepareEgress(context.Background(), s, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildConfig(pinned); err != nil || calls != 1 || pinned.Inbounds[0].Egress.Address != "8.8.8.8" {
		t.Fatalf("config generation repeated DNS or lost pinned IP: %v", err)
	}
	if _, err := prepareEgress(context.Background(), s, lookup); err == nil || calls != 2 {
		t.Fatal("a fresh preparation accepted rebound private DNS")
	}
	if pinned.Inbounds[0].Egress.Address != "8.8.8.8" || s.Inbounds[0].Egress.Address != "exit.example.com" {
		t.Fatal("rebinding changed the previously pinned or desired spec")
	}
	for _, address := range []string{"8.8.8.8.", "::ffff:8.8.8.8", "2606:4700:4700::1111"} {
		s.Inbounds[0].Egress.Address = address
		if _, err := prepareEgress(context.Background(), s, func(context.Context, string) ([]netip.Addr, error) {
			t.Fatal("IP literal caused a DNS lookup")
			return nil, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrepareEgressDefaultResolverRejectsPrivateDNS(t *testing.T) {
	domainTestDNS(t)
	if _, err := prepareEgress(context.Background(), egressResolveSpec(t), nil); err == nil || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("the production DNS resolver accepted a private upstream: %v", err)
	}
}

func TestPrepareEgressBoundsWorkAndSharesContext(t *testing.T) {
	t.Run("host-limit", func(t *testing.T) {
		s := egressResolveSpec(t)
		base := s.Inbounds[0]
		s.Inbounds = nil
		for i := 0; i <= egressMaxHosts; i++ {
			in := base
			e := *base.Egress
			e.Address = fmt.Sprintf("exit-%d.example.com", i)
			in.Egress = &e
			s.Inbounds = append(s.Inbounds, in)
		}
		calls := 0
		_, err := prepareEgress(context.Background(), s, func(context.Context, string) ([]netip.Addr, error) {
			calls++
			return egressAnswerIPs("1.1.1.1"), nil
		})
		if err == nil || calls > egressMaxHosts {
			t.Fatalf("too much DNS work: calls=%d err=%v", calls, err)
		}
	})
	t.Run("reference-limit", func(t *testing.T) {
		s := egressResolveSpec(t)
		s.Inbounds = make([]InboundSpec, egressMaxReferences+1)
		for i := range s.Inbounds {
			s.Inbounds[i].Egress = hy2TestEgress(t, "")
		}
		_, err := prepareEgress(context.Background(), s, func(context.Context, string) ([]netip.Addr, error) {
			t.Fatal("oversized desired spec started DNS work")
			return nil, nil
		})
		if err == nil {
			t.Fatal("unbounded number of egress references accepted")
		}
	})
	t.Run("shared-deadline", func(t *testing.T) {
		type contextKey struct{}
		ctx := context.WithValue(context.Background(), contextKey{}, "preserved")
		ctx, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
		defer cancel()
		wantDeadline, _ := ctx.Deadline()
		s := egressResolveSpec(t)
		route := *s.Inbounds[0].Egress
		route.Address = "another.example.com"
		s.Inbounds[0].DomainRoutes = []DomainRouteSpec{{Domain: "example.net", Match: "suffix", Egress: &route}}
		calls := 0
		start := time.Now()
		_, err := prepareEgress(ctx, s, func(ctx context.Context, host string) ([]netip.Addr, error) {
			deadline, ok := ctx.Deadline()
			if !ok || !deadline.Equal(wantDeadline) || ctx.Value(contextKey{}) != "preserved" {
				t.Fatal("lookup lost the parent context or received a renewed deadline")
			}
			calls++
			if calls == 1 {
				return egressAnswerIPs("1.1.1.1"), nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		})
		if !errors.Is(err, context.DeadlineExceeded) || calls != 2 || time.Since(start) > time.Second || !strings.Contains(err.Error(), "网站规则 1") || !strings.Contains(err.Error(), "another.example.com") {
			t.Fatalf("deadline was not bounded/actionable: calls=%d err=%v", calls, err)
		}
	})
	t.Run("default-deadline-and-cancel", func(t *testing.T) {
		start := time.Now()
		_, err := prepareEgress(context.Background(), egressResolveSpec(t), func(ctx context.Context, _ string) ([]netip.Addr, error) {
			deadline, ok := ctx.Deadline()
			if !ok || deadline.After(start.Add(egressResolveTimeout+time.Second)) {
				t.Fatal("lookup has no bounded overall deadline")
			}
			return nil, context.Canceled
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation was not preserved: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := prepareEgress(ctx, egressResolveSpec(t), func(context.Context, string) ([]netip.Addr, error) {
			t.Fatal("canceled apply performed DNS work")
			return nil, nil
		}); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled apply was accepted: %v", err)
		}
	})
}

func TestPublicEgressAnswerRejectsManagedSelfEndpoints(t *testing.T) {
	local := netip.MustParseAddr("8.8.8.8")
	localIPs, ports := map[netip.Addr]bool{local: true}, map[int]bool{24443: true}
	for _, answers := range [][]netip.Addr{{local}, egressAnswerIPs("1.1.1.1", "::ffff:8.8.8.8")} {
		if _, err := publicEgressAnswer(answers, 24443, localIPs, ports); err == nil {
			t.Fatal("managed-self endpoint hidden in DNS answers was accepted")
		}
		if _, err := publicEgressAnswer(answers, 443, localIPs, ports); err != nil {
			t.Fatalf("unrelated local public port was incorrectly treated as a managed loop: %v", err)
		}
	}
}

func TestFeatureEgressResolutionRunnerRetainsHealthyConfig(t *testing.T) {
	binary := os.Getenv("XRAY_FEATURE_BINARY")
	if binary == "" {
		t.Skip("set XRAY_FEATURE_BINARY for isolated real-core verification")
	}
	for _, metered := range []bool{false, true} {
		t.Run(fmt.Sprintf("metered-%t", metered), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Symlink(binary, filepath.Join(dir, "xray")); err != nil {
				t.Fatal(err)
			}
			r := NewRunner(dir, "unused-egress-dns-test")
			r.systemd = false
			t.Cleanup(r.Stop)
			answers := egressAnswerIPs("1.1.1.1")
			calls := 0
			r.egressLookup = func(context.Context, string) ([]netip.Addr, error) {
				calls++
				return answers, nil
			}
			s := egressResolveSpec(t)
			s.Version, s.APIPort = DefaultVersion, featurePort(t)
			s.Inbounds[0].Port = featurePort(t)
			private, _, err := GenerateRealityKeys()
			if err != nil {
				t.Fatal(err)
			}
			s.Inbounds[0].PrivateKey = private
			if metered {
				s.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
			}
			originalRev := s.Rev()
			if err := r.Apply(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			oldProcess, oldCollector := r.cmd.Process.Pid, r.website
			oldConfig, err := os.ReadFile(r.configPath())
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || r.Rev() != originalRev || s.Rev() != originalRev || s.Inbounds[0].Egress.Address != "exit.example.com" || !bytes.Contains(oldConfig, []byte(`"address": "1.1.1.1"`)) || !bytes.Contains(oldConfig, []byte(`"serverName": "exit.example.com"`)) {
				t.Fatal("Runner did not apply a pinned runtime copy under the desired revision")
			}
			assertRetained := func() {
				t.Helper()
				config, err := os.ReadFile(r.configPath())
				if running, _ := r.Status(); !running || err != nil || !bytes.Equal(config, oldConfig) || r.Rev() != originalRev || r.cmd.Process.Pid != oldProcess || r.website != oldCollector {
					t.Fatal("DNS change interrupted the healthy process, collector, revision or config")
				}
			}
			answers = egressAnswerIPs("127.0.0.1")
			if err := r.Apply(context.Background(), s); err != nil || calls != 1 {
				t.Fatalf("unchanged desired config re-resolved DNS: calls=%d err=%v", calls, err)
			}
			assertRetained()
			changed := s
			e := *s.Inbounds[0].Egress
			e.Password = "new-secret"
			changed.Inbounds = append([]InboundSpec(nil), s.Inbounds...)
			changed.Inbounds[0].Egress = &e
			for _, unsafe := range [][]netip.Addr{egressAnswerIPs("127.0.0.1"), egressAnswerIPs("1.1.1.1", "10.0.0.1")} {
				answers = unsafe
				err := r.Apply(context.Background(), changed)
				if err == nil || !strings.Contains(err.Error(), "exit.example.com") || !strings.Contains(err.Error(), "in-7") || strings.Contains(err.Error(), "new-secret") {
					t.Fatalf("unsafe new DNS did not produce a safe actionable error: %v", err)
				}
				assertRetained()
				if _, lastErr := r.Status(); lastErr == "" {
					t.Fatal("failed DNS apply was not reported")
				}
			}
			badRoute := changed
			badRoute.Inbounds = append([]InboundSpec(nil), changed.Inbounds...)
			defaultExit, ruleExit := e, e
			defaultExit.Address, ruleExit.Address = "1.1.1.1", "rule-exit.example.com"
			badRoute.Inbounds[0].Egress = &defaultExit
			badRoute.Inbounds[0].DomainRoutes = []DomainRouteSpec{{Domain: "example.net", Match: "suffix", Egress: &ruleExit}}
			if err := r.Apply(context.Background(), badRoute); err == nil || !strings.Contains(err.Error(), "网站规则 1") || !strings.Contains(err.Error(), "rule-exit.example.com") {
				t.Fatalf("unsafe website exit was accepted or not identified: %v", err)
			}
			assertRetained()
			beforeCalls := calls
			if err := r.Apply(context.Background(), s); err != nil || calls != beforeCalls {
				t.Fatalf("reverting to the running desired revision performed DNS work: %v", err)
			}
			if _, lastErr := r.Status(); lastErr != "" {
				t.Fatalf("healthy reverted configuration retained a stale DNS error: %s", lastErr)
			}
			answers = egressAnswerIPs("9.9.9.9")
			if err := r.Apply(context.Background(), changed); err != nil || r.Rev() != changed.Rev() || e.Address != "exit.example.com" || r.cmd.Process.Pid == oldProcess {
				t.Fatalf("public DNS recovery did not apply the new desired revision: %v", err)
			}
			config, _ := os.ReadFile(r.configPath())
			if !bytes.Contains(config, []byte(`"address": "9.9.9.9"`)) {
				t.Fatal("recovery did not pin the newly validated IP")
			}
		})
	}
}
