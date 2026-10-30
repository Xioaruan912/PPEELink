package api

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"ppeelink/models"
	"ppeelink/xraycore"
	"strconv"
	"strings"
	"sync"
	"time"
)

const forwardDNSCacheLimit = 256
const forwardDNSCacheTTL = 45 * time.Second

var forwardLookupIP = func(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

type forwardDNSCacheEntry struct {
	addresses []netip.Addr
	expires   time.Time
}

var forwardDNSCache = struct {
	sync.Mutex
	entries map[string]forwardDNSCacheEntry
}{entries: map[string]forwardDNSCacheEntry{}}

func normalizeForwardHost(host string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap().String()
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

func cachedForwardIPs(ctx context.Context, host string) []netip.Addr {
	now := time.Now()
	forwardDNSCache.Lock()
	if entry, ok := forwardDNSCache.entries[host]; ok && now.Before(entry.expires) {
		forwardDNSCache.Unlock()
		return entry.addresses
	}
	forwardDNSCache.Unlock()
	ips, _ := forwardLookupIP(ctx, host)
	public := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if relayPublicIP(ip) {
			public = append(public, ip.Unmap())
		}
	}
	forwardDNSCache.Lock()
	defer forwardDNSCache.Unlock()
	// Expire old entries first and evict the oldest remaining entry if full.
	for key, entry := range forwardDNSCache.entries {
		if !now.Before(entry.expires) {
			delete(forwardDNSCache.entries, key)
		}
	}
	if len(forwardDNSCache.entries) >= forwardDNSCacheLimit {
		var oldest string
		var expiry time.Time
		for key, entry := range forwardDNSCache.entries {
			if oldest == "" || entry.expires.Before(expiry) {
				oldest, expiry = key, entry.expires
			}
		}
		delete(forwardDNSCache.entries, oldest)
	}
	forwardDNSCache.entries[host] = forwardDNSCacheEntry{public, time.Now().Add(forwardDNSCacheTTL)}
	return public
}

func forwardHostAliases(hosts map[string]bool) map[string][]string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out := map[string][]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := make(chan struct{}, 8)
	for host := range hosts {
		if host != "" {
			out[host] = []string{host}
		}
	}
	for host := range hosts {
		if host == "" {
			continue
		}
		if _, err := netip.ParseAddr(host); err == nil {
			continue
		}
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			select {
			case workers <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-workers }()
			identities := []string{host}
			for _, ip := range cachedForwardIPs(ctx, host) {
				identities = append(identities, ip.String())
			}
			mu.Lock()
			out[host] = identities
			mu.Unlock()
		}(host)
	}
	wg.Wait()
	return out
}

// The graph includes opaque lines and authenticated gateways. It validates the
// proposed final state, so a cloned/manual node cannot evade source-ID checks.
func validateManagedForwardGraph(relay *models.AgentRelay, inbound *models.ProxyInbound) error {
	var servers []models.ProxyServer
	if e := models.DB.Find(&servers).Error; e != nil {
		return e
	}
	byServer := map[uint]models.ProxyServer{}
	for _, s := range servers {
		byServer[s.ID] = s
	}
	var relays []models.AgentRelay
	if relayTables() {
		if e := models.DB.Find(&relays).Error; e != nil {
			return e
		}
	}
	if relay != nil {
		found := false
		for i := range relays {
			if relay.ID != 0 && relays[i].ID == relay.ID {
				relays[i] = *relay
				found = true
			}
		}
		if !found {
			relays = append(relays, *relay)
		}
	}
	var inbounds []models.ProxyInbound
	if e := models.DB.Find(&inbounds).Error; e != nil {
		return e
	}
	if inbound != nil {
		found := false
		for i := range inbounds {
			if inbound.ID != 0 && inbounds[i].ID == inbound.ID {
				inbounds[i] = *inbound
				found = true
			}
		}
		if !found {
			inbounds = append(inbounds, *inbound)
		}
	}
	type endpoint struct {
		host string
		port int
	}
	type edge struct{ from, to endpoint }
	rawEdges := []edge{}
	hosts := map[string]bool{}
	addHost := func(host string) { hosts[normalizeForwardHost(host)] = true }
	nodeEndpoint := func(id int) endpoint {
		var n models.Node
		if models.DB.First(&n, id).Error != nil {
			return endpoint{}
		}
		u, e := url.Parse(n.Link)
		if e != nil {
			return endpoint{}
		}
		p, _ := strconv.Atoi(u.Port())
		if p < 1 || p > 65535 {
			return endpoint{}
		}
		return endpoint{normalizeForwardHost(u.Hostname()), p}
	}
	addEdge := func(from endpoint, to endpoint) {
		if to.host == "" || to.port == 0 {
			return
		}
		rawEdges = append(rawEdges, edge{from, to})
		addHost(from.host)
		addHost(to.host)
	}
	for _, r := range relays {
		from := endpoint{normalizeForwardHost(byServer[r.ServerID].PublicHost), r.ListenPort}
		forTarget := nodeEndpoint(r.TargetNodeID)
		addEdge(from, forTarget)
	}
	for _, in := range inbounds {
		from := endpoint{normalizeForwardHost(byServer[in.ServerID].PublicHost), in.Port}
		for _, id := range inboundExitIDs(in) {
			addEdge(from, nodeEndpoint(id))
		}
	}
	// Resolve all identities within one shared budget. A DNS failure does not
	// invalidate an unrelated line; the Agent validates and pins actual dials.
	aliases := forwardHostAliases(hosts)
	keys := func(ep endpoint) []string {
		host := normalizeForwardHost(ep.host)
		identities := aliases[host]
		if len(identities) == 0 {
			identities = []string{host}
		}
		out := make([]string, 0, len(identities))
		for _, identity := range identities {
			out = append(out, net.JoinHostPort(identity, strconv.Itoa(ep.port)))
		}
		return out
	}
	edges := map[string][]string{}
	for _, e := range rawEdges {
		for _, from := range keys(e.from) {
			edges[from] = append(edges[from], keys(e.to)...)
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var walk func(string, int) error
	walk = func(k string, depth int) error {
		if visiting[k] || depth > 32 {
			return fmt.Errorf("中转或网关出口路径存在循环或过长")
		}
		if done[k] {
			return nil
		}
		visiting[k] = true
		for _, to := range edges[k] {
			if e := walk(to, depth+1); e != nil {
				return e
			}
		}
		delete(visiting, k)
		done[k] = true
		return nil
	}
	// Only validate paths reachable from the edited/dispatching listener; an
	// unrelated broken line must not prevent healthy routes from being managed.
	if relay != nil {
		for _, k := range keys(endpoint{byServer[relay.ServerID].PublicHost, relay.ListenPort}) {
			if e := walk(k, 0); e != nil {
				return e
			}
		}
		return nil
	}
	if inbound != nil {
		for _, k := range keys(endpoint{byServer[inbound.ServerID].PublicHost, inbound.Port}) {
			if e := walk(k, 0); e != nil {
				return e
			}
		}
		return nil
	}
	for from := range edges {
		if e := walk(from, 0); e != nil {
			return e
		}
	}
	return nil
}
func requireGatewayCapability(e *xraycore.EgressSpec, routes []xraycore.DomainRouteSpec, s models.ProxyServer) error {
	if s.Kind == models.ServerKindAgent && s.ProxyFeatures < 6 {
		insecure := e != nil && e.Insecure
		for _, route := range routes {
			insecure = insecure || (route.Egress != nil && route.Egress.Insecure)
		}
		if insecure {
			return fmt.Errorf("请先升级 Agent 至1.12，以使用 x-ui 自签名 HY2 出口")
		}
	}
	if s.Kind != models.ServerKindAgent || s.ProxyFeatures >= 5 {
		return nil
	}
	if e != nil && e.IsHY2() {
		return fmt.Errorf("请先升级 Agent 以支持 HY2 落地出口")
	}
	for _, r := range routes {
		if r.Egress != nil && r.Egress.IsHY2() {
			return fmt.Errorf("请先升级 Agent 以支持 HY2 落地出口")
		}
	}
	return nil
}
