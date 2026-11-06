package node

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMieruClashExportIncludesConnectionSettings(t *testing.T) {
	body, err := EncodeClashProxies([]string{testMierusLink + "#IPLC_HK"}, nil, SqlConfig{Udp: true})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Proxies) != 1 {
		t.Fatalf("Mieru node disappeared from subscription: %d proxies", len(cfg.Proxies))
	}
	for key, want := range map[string]any{"name": "IPLC_HK", "type": "mieru", "server": "192.0.2.1", "port": 6901, "username": "primary", "password": "example-secret", "transport": "TCP", "multiplexing": "MULTIPLEXING_OFF", "handshake-mode": "HANDSHAKE_NO_WAIT", "udp": true} {
		if cfg.Proxies[0][key] != want {
			t.Errorf("Mieru export lost or changed field %s", key)
		}
	}
}

func TestMieruClashExportsEachPortProtocolBinding(t *testing.T) {
	link := "mierus://primary:example-secret@[2001:db8::1]?port=6901-6903&port=7901&protocol=TCP&protocol=UDP&profile=default#Mieru"
	body, err := EncodeClashProxies([]string{link}, map[string]NodeFlags{link: {TFO: true}}, SqlConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Proxies) != 2 {
		t.Fatal("multiple port/protocol bindings were dropped")
	}
	a, b := cfg.Proxies[0], cfg.Proxies[1]
	if a["port-range"] != "6901-6903" || a["port"] != nil || a["transport"] != "TCP" || b["port"] != 7901 || b["port-range"] != nil || b["transport"] != "UDP" || a["name"] == b["name"] || a["tfo"] != true || b["tfo"] != true {
		t.Fatal("port ranges, transports, names or per-node flags were not preserved")
	}
}

func TestMieruCompatibilityOnlyIncludesImplementedClients(t *testing.T) {
	p, err := ParseOutbound(testMierusLink)
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"clash", "clash-proxies", "stash", "singbox", "surge", "loon", "qx"} {
		status, _ := checkClient(p, client)
		wantSupported := client == "clash" || client == "clash-proxies"
		if (status != "unsupported") != wantSupported {
			t.Errorf("client %s received incorrect Mieru support status: %s", client, status)
		}
	}
	for _, link := range []string{
		strings.Replace(testMierusLink, "mtu=1400", "mtu=1280", 1),
		strings.Replace(testMierusLink, "MULTIPLEXING_OFF", "INVALID_LEVEL", 1),
		strings.Replace(testMierusLink, "HANDSHAKE_NO_WAIT", "INVALID_MODE", 1),
	} {
		p, err := ParseOutbound(link)
		if err != nil {
			t.Fatal(err)
		}
		if status, _ := checkClient(p, "clash"); status != "unsupported" {
			t.Error("unrepresentable Mieru options were silently accepted for Mihomo")
		}
	}
}

func TestMieruExpandedNamesDoNotCollideWithExistingNodes(t *testing.T) {
	multi := "mierus://primary:example-secret@192.0.2.1?port=6901&port=7901&protocol=TCP&protocol=UDP&profile=default#Mieru"
	proxies := BuildClashProxies([]string{multi, testMierusLink + "#Mieru%20[1]", testMierusLink + "#Mieru%20[1]%20%232"}, nil, SqlConfig{})
	if len(proxies) != 4 {
		t.Fatalf("expected all four proxies, got %d", len(proxies))
	}
	names := map[string]bool{}
	for _, p := range proxies {
		if names[p.Name] {
			t.Fatal("expanded Mieru binding collides with an existing node name")
		}
		names[p.Name] = true
	}
	if !names["Mieru [1]"] || !names["Mieru [1] #2"] {
		t.Fatal("existing node names were changed")
	}
}
