package xraycore

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDomainRouteValidation(t *testing.T) {
	for _, bad := range []string{"https://example.com", "*.example.com", "example.com:443", "127.0.0.1", "a..com", "-a.com", "com", "a.com/path"} {
		if _, e := NormalizeRouteDomain(bad); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	if d, e := NormalizeRouteDomain(" Example.COM. "); e != nil || d != "example.com" {
		t.Fatalf("normalize %s %v", d, e)
	}
	if d, e := NormalizeRouteDomain("例子.测试"); e != nil || !strings.HasPrefix(d, "xn--") {
		t.Fatalf("IDNA %s %v", d, e)
	}
}
func TestDomainRoutesConfigOrderingAndMetering(t *testing.T) {
	s := websiteTestSpec()
	s.WebsiteTraffic = nil
	s.Inbounds[0].DomainRoutes = []DomainRouteSpec{{Domain: "api.example.com", Match: "exact"}, {Domain: "example.com", Match: "suffix"}}
	for _, metered := range []bool{false, true} {
		var c *WebsiteCollector
		if metered {
			s.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
			c = websiteTestCollector(t, s)
		}
		raw, e := BuildConfigWithWebsiteCollector(s, c)
		if e != nil {
			t.Fatal(e)
		}
		var doc map[string]any
		json.Unmarshal(raw, &doc)
		rules := doc["routing"].(map[string]any)["rules"].([]any)
		matches := []string{}
		for _, v := range rules {
			m := v.(map[string]any)
			if d, ok := m["domain"]; ok {
				matches = append(matches, d.([]any)[0].(string))
				if metered && !strings.HasPrefix(m["outboundTag"].(string), "website-") {
					t.Fatal("rule bypassed meter")
				}
			}
		}
		if strings.Join(matches, ",") != "full:api.example.com,domain:example.com" {
			t.Fatalf("order %v", matches)
		}
		if metered {
			for _, id := range c.identities {
				if strings.HasPrefix(id.tag, "route-") && c.statsTag(id.tag) != "in-7" {
					t.Fatal("lost original inbound attribution")
				}
			}
		}
	}
}

func TestCategoryDomainRoutes(t *testing.T) {
	if value, err := NormalizeRouteCategory(" GOOGLE "); err != nil || value != "google" {
		t.Fatalf("normalization: %q %v", value, err)
	}
	for _, bad := range []string{"../../secret", "other", "google@ads", "geosite:google", ""} {
		if _, err := NormalizeRouteCategory(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	r := DomainRouteSpec{Match: "category", Domain: "google"}
	p, err := r.domainPattern()
	if err != nil || p != "geosite:google" {
		t.Fatalf("category pattern %q %v", p, err)
	}
	s := websiteTestSpec()
	s.Inbounds[0].DomainRoutes = []DomainRouteSpec{{Match: "exact", Domain: "google.com"}, r}
	for _, metered := range []bool{false, true} {
		var c *WebsiteCollector
		s.WebsiteTraffic = nil
		if metered {
			s.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
			c = websiteTestCollector(t, s)
		}
		raw, err := BuildConfigWithWebsiteCollector(s, c)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "geosite:google") || strings.Index(string(raw), "full:google.com") > strings.Index(string(raw), "geosite:google") {
			t.Fatal("category order changed")
		}
	}
}
