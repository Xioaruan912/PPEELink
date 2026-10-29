package xraycore

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"
)

const (
	egressResolveTimeout = 5 * time.Second
	egressMaxHosts       = 64
	egressMaxAnswers     = 64
	egressMaxReferences  = 1024
)

type egressLookup func(context.Context, string) ([]netip.Addr, error)

// prepareEgress runs only when applying a new runtime configuration, never in
// the pure config builder. Resolve once per canonical host, validate EVERY
// answer, then pin one deterministic IP so Xray cannot resolve it a second time.
// Only the copied runtime spec changes; SNI, credentials and the desired Rev do not.
func prepareEgress(ctx context.Context, spec Spec, lookup egressLookup) (Spec, error) {
	ctx, cancel := context.WithTimeout(ctx, egressResolveTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Spec{}, fmt.Errorf("出口地址准备失败: %w", err)
	}
	type reference struct {
		egress   *EgressSpec
		location string
	}
	refs := []reference{}
	clone := func(e *EgressSpec, location string) (*EgressSpec, error) {
		if e == nil {
			return nil, nil
		}
		if len(refs) >= egressMaxReferences {
			return nil, fmt.Errorf("出口引用最多 %d 个", egressMaxReferences)
		}
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", location, err)
		}
		copy := *e
		copy.ALPN = slices.Clone(e.ALPN)
		refs = append(refs, reference{&copy, location})
		return &copy, nil
	}
	spec.Inbounds = slices.Clone(spec.Inbounds)
	for i := range spec.Inbounds {
		if err := ctx.Err(); err != nil {
			return Spec{}, fmt.Errorf("出口地址准备失败: %w", err)
		}
		in := &spec.Inbounds[i]
		if err := validateDomainRoutes(in.DomainRoutes); err != nil {
			return Spec{}, fmt.Errorf("入站 %q: %w", in.Tag, err)
		}
		var err error
		in.Egress, err = clone(in.Egress, fmt.Sprintf("入站 %q 默认出口", in.Tag))
		if err != nil {
			return Spec{}, err
		}
		in.Users = slices.Clone(in.Users)
		in.DomainRoutes = slices.Clone(in.DomainRoutes)
		for j := range in.DomainRoutes {
			r := &in.DomainRoutes[j]
			r.Egress, err = clone(r.Egress, fmt.Sprintf("入站 %q 网站规则 %d (%s)", in.Tag, j+1, r.Domain))
			if err != nil {
				return Spec{}, err
			}
		}
	}
	if len(refs) == 0 {
		return spec, nil
	}
	// Detect local public addresses pointing back at a managed listener too.
	// Private/local DNS answers are rejected regardless of the requested port.
	interfaces, err := net.InterfaceAddrs()
	if err != nil {
		return Spec{}, fmt.Errorf("读取本机地址以检查出口循环失败: %w", err)
	}
	localIPs := map[netip.Addr]bool{}
	for _, a := range interfaces {
		if prefix, err := netip.ParsePrefix(a.String()); err == nil {
			localIPs[prefix.Addr().Unmap()] = true
		}
	}
	managedPorts := map[int]bool{spec.APIPort: true}
	for _, in := range spec.Inbounds {
		managedPorts[in.Port] = true
	}
	for _, r := range spec.Relays {
		managedPorts[r.ListenPort] = true
	}
	if lookup == nil {
		lookup = func(ctx context.Context, host string) ([]netip.Addr, error) {
			// Use absolute DNS names, without local search-domain expansion.
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host+".")
		}
	}
	answers := map[string][]netip.Addr{}
	for _, ref := range refs {
		e := ref.egress
		host := strings.ToLower(strings.TrimSuffix(e.Address, "."))
		if ip, err := netip.ParseAddr(host); err == nil {
			host = ip.Unmap().String()
		}
		ips, ok := answers[host]
		if !ok {
			if len(answers) >= egressMaxHosts {
				return Spec{}, fmt.Errorf("出口地址最多 %d 个不同主机", egressMaxHosts)
			}
			if err := ctx.Err(); err != nil {
				return Spec{}, fmt.Errorf("%s %q: 出口 DNS 解析失败: %w", ref.location, host, err)
			}
			if ip, err := netip.ParseAddr(host); err == nil {
				ips = []netip.Addr{ip}
			} else {
				ips, err = lookup(ctx, host)
				if err != nil {
					return Spec{}, fmt.Errorf("%s %q: 出口 DNS 解析失败: %w", ref.location, host, err)
				}
			}
			answers[host] = ips
		}
		if err := ctx.Err(); err != nil {
			return Spec{}, fmt.Errorf("%s %q: 出口 DNS 解析失败: %w", ref.location, host, err)
		}
		ip, err := publicEgressAnswer(ips, e.Port, localIPs, managedPorts)
		if err != nil {
			return Spec{}, fmt.Errorf("%s %q: %w", ref.location, host, err)
		}
		e.Address = ip.String()
	}
	return spec, nil
}

func publicEgressAnswer(ips []netip.Addr, port int, localIPs map[netip.Addr]bool, managedPorts map[int]bool) (netip.Addr, error) {
	if len(ips) == 0 || len(ips) > egressMaxAnswers {
		return netip.Addr{}, fmt.Errorf("出口 DNS 应返回 1 到 %d 个公网地址", egressMaxAnswers)
	}
	var chosen netip.Addr
	for _, ip := range ips {
		if !ip.IsValid() || ip.Zone() != "" {
			return netip.Addr{}, fmt.Errorf("出口 DNS 返回无效地址")
		}
		ip = ip.Unmap()
		if websitePrivateIP(net.IP(ip.AsSlice())) || egressReservedV4.Contains(ip) || egressReservedV6.Contains(ip) {
			return netip.Addr{}, fmt.Errorf("出口 DNS 返回非公网地址 %s，请检查落地域名解析", ip)
		}
		if localIPs[ip] && managedPorts[port] {
			return netip.Addr{}, fmt.Errorf("出口 %s:%d 指向本机受管监听端口，请选择远端落地", ip, port)
		}
		// Prefer IPv4, then the lowest IP, independent of DNS answer order.
		if !chosen.IsValid() || ip.Compare(chosen) < 0 {
			chosen = ip
		}
	}
	return chosen, nil
}

var (
	egressReservedV4 = netip.MustParsePrefix("0.0.0.0/8")
	egressReservedV6 = netip.MustParsePrefix("2001::/23")
)
