package api

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"ppeelink/models"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagedForwardGraphNormalizesEndpointSpellings(t *testing.T) {
	for _, tc := range []struct{ name, server, alias string }{
		{"trailing-dot", "relay.example.com", "RELAY.EXAMPLE.COM."},
		{"ipv6", "2606:4700:4700::1111", "2606:4700:4700:0:0:0:0:1111"},
		{"mapped-ipv4", "8.8.8.8", "::ffff:8.8.8.8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, n := setupRelayDB(t)
			s.PublicHost = tc.server
			if err := models.DB.Save(&s).Error; err != nil {
				t.Fatal(err)
			}
			models.DB.Model(&n).Update("link", strings.Replace(n.Link, "1.1.1.1:443", net.JoinHostPort(tc.alias, strconv.Itoa(25001)), 1))
			r := models.AgentRelay{ServerID: s.ID, Name: "loop", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
			if err := models.DB.Create(&r).Error; err != nil {
				t.Fatal(err)
			}
			if err := validateManagedForwardGraph(&r, nil); err == nil {
				t.Fatal("equivalent endpoint loop accepted")
			}
		})
	}
}

func withForwardDNS(t *testing.T, lookup func(context.Context, string) ([]netip.Addr, error)) {
	t.Helper()
	old := forwardLookupIP
	forwardDNSCache.Lock()
	saved := forwardDNSCache.entries
	forwardDNSCache.entries = map[string]forwardDNSCacheEntry{}
	forwardDNSCache.Unlock()
	forwardLookupIP = lookup
	t.Cleanup(func() {
		forwardLookupIP = old
		forwardDNSCache.Lock()
		forwardDNSCache.entries = saved
		forwardDNSCache.Unlock()
	})
}

func TestManagedForwardGraphDetectsDNSAliasLoop(t *testing.T) {
	for _, domainOnServer := range []bool{false, true} {
		t.Run(fmt.Sprint(domainOnServer), func(t *testing.T) {
			var lookups atomic.Int32
			withForwardDNS(t, func(ctx context.Context, host string) ([]netip.Addr, error) {
				lookups.Add(1)
				if host == "entry.example.com" {
					return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
				}
				return nil, fmt.Errorf("unexpected host %s", host)
			})
			a, n := setupRelayDB(t)
			if domainOnServer {
				a.PublicHost = "entry.example.com"
				models.DB.Save(&a)
			}
			b := models.ProxyServer{Name: "B", Kind: models.ServerKindAgent, PublicHost: "9.9.9.9", Enabled: true, ProxyFeatures: 5}
			models.DB.Create(&b)
			models.DB.Model(&n).Update("link", strings.Replace(n.Link, "1.1.1.1:443", "9.9.9.9:25002", 1))
			target := "entry.example.com:25001"
			if domainOnServer {
				target = "8.8.8.8:25001"
			}
			toA := models.Node{Name: "alias A", Link: strings.Replace(n.Link, "9.9.9.9:25002", target, 1)}
			models.DB.Create(&toA)
			ra := models.AgentRelay{ServerID: a.ID, Name: "A", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
			rb := models.AgentRelay{ServerID: b.ID, Name: "B", ListenPort: 25002, Network: "tcp", TargetNodeID: toA.ID, Enabled: true}
			models.DB.Create(&ra)
			models.DB.Create(&rb)
			for i := 0; i < 2; i++ {
				if err := validateManagedForwardGraph(&ra, nil); err == nil {
					t.Fatal("DNS alias cycle accepted")
				}
			}
			if lookups.Load() != 1 {
				t.Fatalf("repeated graph walks re-resolved cache: %d", lookups.Load())
			}
		})
	}
}

func TestManagedForwardGraphDNSFailureDoesNotBlockUnrelatedPath(t *testing.T) {
	withForwardDNS(t, func(context.Context, string) ([]netip.Addr, error) { return nil, fmt.Errorf("DNS unavailable") })
	a, n := setupRelayDB(t)
	r := models.AgentRelay{ServerID: a.ID, Name: "good", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	models.DB.Create(&r)
	b := models.ProxyServer{Name: "unrelated", Kind: models.ServerKindAgent, PublicHost: "unavailable.example.com", Enabled: true, ProxyFeatures: 5}
	models.DB.Create(&b)
	bad := models.AgentRelay{ServerID: b.ID, Name: "unrelated", ListenPort: 25002, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	models.DB.Create(&bad)
	if err := validateManagedForwardGraph(&r, nil); err != nil {
		t.Fatalf("unrelated failed DNS blocked healthy path: %v", err)
	}
}

func TestForwardDNSCacheExpiresAndStaysBounded(t *testing.T) {
	var lookups atomic.Int32
	withForwardDNS(t, func(context.Context, string) ([]netip.Addr, error) {
		lookups.Add(1)
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	})
	ctx := context.Background()
	cachedForwardIPs(ctx, "stale.example.com")
	forwardDNSCache.Lock()
	entry := forwardDNSCache.entries["stale.example.com"]
	entry.expires = time.Now().Add(-time.Second)
	forwardDNSCache.entries["stale.example.com"] = entry
	forwardDNSCache.Unlock()
	cachedForwardIPs(ctx, "stale.example.com")
	if lookups.Load() != 2 {
		t.Fatal("expired DNS entry was reused")
	}
	for i := 0; i < forwardDNSCacheLimit+10; i++ {
		cachedForwardIPs(ctx, fmt.Sprintf("host-%d.example.com", i))
	}
	forwardDNSCache.Lock()
	count := len(forwardDNSCache.entries)
	forwardDNSCache.Unlock()
	if count > forwardDNSCacheLimit {
		t.Fatalf("DNS cache grew to %d", count)
	}
}

func TestForwardDNSResolutionHasSharedBudget(t *testing.T) {
	withForwardDNS(t, func(ctx context.Context, host string) ([]netip.Addr, error) { <-ctx.Done(); return nil, ctx.Err() })
	hosts := map[string]bool{}
	for i := 0; i < 32; i++ {
		hosts[fmt.Sprintf("host-%d.example.com", i)] = true
	}
	started := time.Now()
	aliases := forwardHostAliases(hosts)
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("resolution exceeded shared budget: %s", elapsed)
	}
	if len(aliases) != len(hosts) {
		t.Fatal("timed-out lookup lost lexical identities")
	}
}
