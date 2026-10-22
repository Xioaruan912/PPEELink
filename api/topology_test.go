package api

import (
	"encoding/json"
	"ppeelink/models"
	"strings"
	"testing"
)

func TestNetworkTopologyExplicitReferencesAndRedaction(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProxyInbound{}, &models.Monitor{}); err != nil {
		t.Fatal(err)
	}
	relay := models.Node{Name: "Relay", Link: "trojan://SECRET@same-host:443"}
	models.DB.Create(&relay)
	n := models.Node{Name: "自建·same-name", Link: "trojan://SECRET@same-host:443", Flags: `{"dialerProxyId":1}`}
	models.DB.Create(&n)
	// Use the real flags encoder shape rather than inferring from host/name.
	flags := decodeNodeFlags(n.Flags)
	flags.DialerProxyID = relay.ID
	raw, err := json.Marshal(flags)
	if err != nil {
		t.Fatal(err)
	}
	models.DB.Model(&n).Update("flags", string(raw))
	g := models.GroupNode{Name: "自建·same-name", Nodes: []models.Node{n}}
	models.DB.Create(&g)
	s := models.Subcription{Name: "Subscription", Token: "PRIVATE-TOKEN", Config: "PRIVATE-CONFIG", Nodes: []models.Node{relay}, GroupRefs: []models.GroupNode{g}}
	models.DB.Create(&s)
	server := models.ProxyServer{Name: "same-name", PublicHost: "same-host", AgentToken: "PRIVATE-AGENT", Enabled: true}
	models.DB.Create(&server)
	in := models.ProxyInbound{ServerID: server.ID, Remark: "Inbound", PrivateKey: "PRIVATE-KEY", Enabled: true}
	models.DB.Create(&in)
	m := models.Monitor{Name: "Monitor", ProxyServerID: &server.ID, TokenHash: "PRIVATE-MONITOR", SSHPasswordEnc: "PRIVATE-SSH", Enabled: true}
	models.DB.Create(&m)
	rec := jsonRequest(t, NetworkTopology, nil, nil)
	if rec.Code != 200 {
		t.Fatalf("response %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{"SECRET", "PRIVATE-", "same-host", "trojan://"} {
		if strings.Contains(body, secret) {
			t.Fatalf("leaked %q: %s", secret, body)
		}
	}
	var payload struct {
		Data struct {
			Nodes    []topologyNode `json:"nodes"`
			Edges    []topologyEdge `json:"edges"`
			Warnings []string       `json:"warnings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Warnings) != 0 {
		t.Fatal(payload.Data.Warnings)
	}
	relations := map[string]int{}
	for _, e := range payload.Data.Edges {
		relations[e.Relation]++
		if strings.HasPrefix(e.Source, "node:") && strings.HasPrefix(e.Target, "server:") {
			t.Fatal("invented node/server association")
		}
	}
	for _, relation := range []string{"relay", "fixed", "reference", "member", "server", "monitor"} {
		if relations[relation] != 1 {
			t.Fatalf("%s refs: %+v", relation, payload.Data.Edges)
		}
	}
	// A GET-like read must not generate tokens or update any configuration.
	models.DB.Model(&s).Update("token", "")
	jsonRequest(t, NetworkTopology, nil, nil)
	var saved models.Subcription
	models.DB.First(&saved, s.ID)
	if saved.Token != "" {
		t.Fatal("topology generated a subscription token")
	}
}

func TestNetworkTopologyPartialSnapshot(t *testing.T) {
	setupTestDB(t)
	n := models.Node{Name: "Retained node"}
	models.DB.Create(&n)
	rec := jsonRequest(t, NetworkTopology, nil, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Retained node") || !strings.Contains(rec.Body.String(), "读取失败") {
		t.Fatalf("expected usable partial data: %s", rec.Body.String())
	}
}
