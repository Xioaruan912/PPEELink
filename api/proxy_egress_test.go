package api

import (
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/xraycore"
	"testing"
)

func TestEgressSelectionAndCapability(t *testing.T) {
	server, in := setupProxyDB(t)
	n := models.Node{Name: "exit", Link: "vless://11111111-1111-4111-8111-111111111111@exit.example.com:24443?security=reality&type=tcp&sni=www.example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&sid=abcd&flow=xtls-rprx-vision"}
	if e := models.DB.Create(&n).Error; e != nil {
		t.Fatal(e)
	}
	in.EgressNodeID = n.ID
	models.DB.Save(&in)
	if _, e := BuildServerSpec(server); e == nil {
		t.Fatal("legacy agent accepted fixed egress")
	}
	server.ProxyFeatures = 1
	if s, e := BuildServerSpec(server); e != nil || s.Inbounds[0].Egress == nil {
		t.Fatalf("valid exit rejected: %v", e)
	}
	server.PublicHost = "exit.example.com"
	models.DB.Save(&server)
	if _, e := resolveInboundEgress(in, server); e == nil {
		t.Fatal("self loop accepted")
	}
	in.EgressNodeID = n.ID + 999
	if _, e := resolveInboundEgress(in, server); e == nil {
		t.Fatal("missing exit accepted")
	}
}
func TestHysteriaLinkAndCapability(t *testing.T) {
	server, in := setupProxyDB(t)
	in.Template = xraycore.TemplateHysteria2
	in.TLSCertFile = "/etc/ssl/cert.pem"
	in.TLSKeyFile = "/etc/ssl/key.pem"
	models.DB.Save(&in)
	if _, e := BuildServerSpec(server); e == nil {
		t.Fatal("legacy agent accepted HY2")
	}
	server.ProxyFeatures = 1
	link := InboundLink(server, in, models.ProxyAccount{UUID: "test-secret"}, "test")
	hy, err := node.DecodeHY2URL(link)
	if err != nil || hy.Password != "test-secret" || hy.Host != server.PublicHost || hy.Sni != in.SNI {
		t.Fatal("invalid HY2 link")
	}
	s, e := BuildServerSpec(server)
	if e != nil || s.Inbounds[0].TLSCertFile != in.TLSCertFile {
		t.Fatalf("missing certificate paths: %v", e)
	}
}
