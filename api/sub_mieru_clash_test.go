package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
	"ppeelink/models"
)

func TestClashSubscriptionIncludesGroupedMieruNode(t *testing.T) {
	sub, group, _ := deliveryFixture(t)
	n := models.Node{Name: "IPLC_HK", Link: "mierus://primary:example-secret@192.0.2.1?handshake-mode=HANDSHAKE_NO_WAIT&mtu=1400&multiplexing=MULTIPLEXING_OFF&port=6901&profile=default&protocol=TCP#IPLC_HK", ForceDeliver: true}
	if err := models.DB.Create(&n).Error; err != nil {
		t.Fatal(err)
	}
	if err := models.DB.Model(&group).Association("Nodes").Append(&n); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "clash.yaml")
	if err := os.WriteFile(path, []byte("proxies: []\nproxy-groups:\n  - name: PROXY\n    type: select\n    proxies: []\nrules:\n  - MATCH,PROXY\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _ := json.Marshal(map[string]any{"clash": path})
	sub.Config = string(cfg)
	if err := models.DB.Model(&sub).Update("config", sub.Config).Error; err != nil {
		t.Fatal(err)
	}
	body, err := buildSubscriptionOutput(sub, "clash", "")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Proxies) != 3 {
		t.Fatalf("subscription lost nodes: got %d, want 3", len(result.Proxies))
	}
	for _, p := range result.Proxies {
		if p["name"] == "IPLC_HK" && p["type"] == "mieru" && p["port"] == 6901 && p["transport"] == "TCP" && p["handshake-mode"] == "HANDSHAKE_NO_WAIT" {
			return
		}
	}
	t.Fatal("grouped Mieru node missing from actual Clash export")
}
