package xraycore

import (
	"fmt"
	"golang.org/x/net/idna"
	"net"
	"strconv"
	"strings"
)

type DomainRouteSpec struct {
	Domain string      `json:"domain"`
	Match  string      `json:"match"`
	Egress *EgressSpec `json:"egress,omitempty"`
}

func NormalizeRouteDomain(value string) (string, error) {
	value = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	host, err := idna.Lookup.ToASCII(value)
	if err != nil || len(host) > 253 || net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return "", fmt.Errorf("请填写有效域名，不含协议、端口或通配符")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("域名格式无效")
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return "", fmt.Errorf("域名格式无效")
			}
		}
	}
	return host, nil
}
func (r DomainRouteSpec) domainPattern() (string, error) {
	domain, err := NormalizeRouteDomain(r.Domain)
	if err != nil {
		return "", err
	}
	switch r.Match {
	case "exact":
		return "full:" + domain, nil
	case "suffix":
		return "domain:" + domain, nil
	default:
		return "", fmt.Errorf("域名匹配方式无效")
	}
}
func validateDomainRoutes(rules []DomainRouteSpec) error {
	if len(rules) > 32 {
		return fmt.Errorf("网站分流最多 32 条规则")
	}
	seen := map[string]bool{}
	for _, r := range rules {
		p, e := r.domainPattern()
		if e != nil {
			return e
		}
		if seen[p] {
			return fmt.Errorf("域名分流规则重复")
		}
		seen[p] = true
		if r.Egress != nil {
			if e := r.Egress.Validate(); e != nil {
				return e
			}
		}
	}
	return nil
}

// Virtual collector identities select the route BEFORE destination resolution.
// The original inbound tag is retained separately for accounting.
type websiteRoute struct{ source, domain string }

func meteredRouteInbounds(inbounds []InboundSpec) ([]InboundSpec, map[string]websiteRoute, error) {
	expanded := []InboundSpec{}
	routes := map[string]websiteRoute{}
	for _, in := range inbounds {
		if e := validateDomainRoutes(in.DomainRoutes); e != nil {
			return nil, nil, e
		}
		for i, r := range in.DomainRoutes {
			v := in
			v.Tag = "route-" + in.Tag + "-" + strconv.Itoa(i)
			v.Egress = r.Egress
			v.DomainRoutes = nil
			expanded = append(expanded, v)
			p, _ := r.domainPattern()
			routes[v.Tag] = websiteRoute{in.Tag, p}
		}
		in.DomainRoutes = nil
		expanded = append(expanded, in)
	}
	return expanded, routes, nil
}
func (c *WebsiteCollector) statsTag(tag string) string {
	if r, ok := c.routes[tag]; ok {
		return r.source
	}
	return tag
}
