package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"ppeelink/models"
	"ppeelink/xraycore"
	"testing"
)

func domainTestExit(t *testing.T, host string, port int) models.Node {
	t.Helper()
	n := models.Node{Name: host, Link: "vless://11111111-1111-4111-8111-111111111111@" + host + ":" + fmt.Sprint(port) + "?security=reality&type=tcp&sni=www.example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&sid=abcd&flow=xtls-rprx-vision"}
	if e := models.DB.Create(&n).Error; e != nil {
		t.Fatal(e)
	}
	return n
}
func TestDomainRoutePersistenceAndGate(t *testing.T) {
	s, in := setupProxyDB(t)
	n := domainTestExit(t, "exit.example.com", 24443)
	in.DomainRoutes = []models.DomainRoute{{Domain: " Example.COM. ", Match: "suffix", EgressNodeID: n.ID}}
	normalized, e := normalizeInboundDomainRoutes(in.DomainRoutes)
	if e != nil || normalized[0].Domain != "example.com" {
		t.Fatalf("normalize %v", e)
	}
	in.DomainRoutes = normalized
	models.DB.Save(&in)
	s.ProxyFeatures = 2
	if _, e = BuildServerSpec(s); e == nil {
		t.Fatal("old Agent accepted routing")
	}
	s.ProxyFeatures = 3
	spec, e := BuildServerSpec(s)
	if e != nil || len(spec.Inbounds[0].DomainRoutes) != 1 || spec.Inbounds[0].DomainRoutes[0].Egress == nil {
		t.Fatalf("missing route %v", e)
	}
	in.DomainRoutes[0].EgressNodeID = 99999
	if _, _, e = resolveInboundRoutes(in, s); e == nil {
		t.Fatal("missing exit accepted")
	}
	if _, e = normalizeInboundDomainRoutes([]models.DomainRoute{{Domain: "example.com", Match: "suffix"}, {Domain: "EXAMPLE.com", Match: "suffix"}}); e == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestDomainRouteConditionalCycles(t *testing.T) {
	s, in := setupProxyDB(t)
	other := models.ProxyServer{Name: "other", Kind: models.ServerKindAgent, PublicHost: "other.example.com", Enabled: true, ProxyFeatures: 3}
	models.DB.Create(&other)
	next := models.ProxyInbound{ServerID: other.ID, Template: xraycore.TemplateRealityVision, Port: 24444, Enabled: true}
	models.DB.Create(&next)
	toOther := domainTestExit(t, other.PublicHost, next.Port)
	back := domainTestExit(t, s.PublicHost, in.Port)
	in.DomainRoutes = []models.DomainRoute{{Domain: "example.com", Match: "suffix", EgressNodeID: toOther.ID}}
	next.DomainRoutes = []models.DomainRoute{{Domain: "example.org", Match: "exact", EgressNodeID: back.ID}}
	models.DB.Save(&next)
	if _, _, e := resolveInboundRoutes(in, s); e == nil {
		t.Fatal("conditional cycle accepted")
	}
	next.DomainRoutes = nil
	models.DB.Save(&next)
	if _, _, e := resolveInboundRoutes(in, s); e != nil {
		t.Fatalf("valid route rejected %v", e)
	}
}

func TestDomainRouteSaveAndRemove(t *testing.T) {
	s, in := setupProxyDB(t)
	s.ProxyFeatures = 2
	models.DB.Save(&s)
	req := inboundSaveRequest{ID: in.ID, ServerID: s.ID, Template: in.Template, Port: in.Port, Target: in.Target, SNI: in.SNI, Enabled: true, DomainRoutes: []models.DomainRoute{{Domain: " Example.COM. ", Match: "suffix", EgressNodeID: 0}}}
	save := func() int {
		raw, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		ProxyInboundSave(c)
		return w.Code
	}
	if save() != 400 {
		t.Fatal("old Agent accepted save")
	}
	s.ProxyFeatures = 3
	models.DB.Save(&s)
	if save() != 200 {
		t.Fatal("valid rule rejected")
	}
	models.DB.First(&in, in.ID)
	if len(in.DomainRoutes) != 1 || in.DomainRoutes[0].Domain != "example.com" {
		t.Fatal("normalized route not persisted")
	}
	req.DomainRoutes = nil
	if save() != 200 {
		t.Fatal("remove rejected")
	}
	var cleared models.ProxyInbound
	models.DB.First(&cleared, in.ID)
	if len(cleared.DomainRoutes) != 0 {
		t.Fatal("removed rules retained")
	}
}

func TestCategoryRouteCapability(t *testing.T) {
	server, in := setupProxyDB(t)
	server.ProxyFeatures = 3
	in.DomainRoutes = []models.DomainRoute{{Domain: " GOOGLE ", Match: "category"}}
	normalized, err := normalizeInboundDomainRoutes(in.DomainRoutes)
	if err != nil || normalized[0].Domain != "google" {
		t.Fatalf("normalize %v", err)
	}
	in.DomainRoutes = normalized
	models.DB.Save(&in)
	if _, err = BuildServerSpec(server); err == nil {
		t.Fatal("old Agent accepted category")
	}
	server.ProxyFeatures = 4
	if spec, err := BuildServerSpec(server); err != nil || spec.Inbounds[0].DomainRoutes[0].Match != "category" {
		t.Fatalf("category failed: %v", err)
	}
}
