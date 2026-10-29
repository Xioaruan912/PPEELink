package diagnostic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestChainConfig(t *testing.T) {
	a, _ := parse("hy2://secret@8.8.8.8:443?sni=example.com")
	b, _ := parse("vless://11111111-1111-4111-8111-111111111111@1.1.1.1:443?security=reality&sni=example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	a.relays = []profile{b}
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	json.Unmarshal(config(a, 12345, "u", "p"), &cfg)
	if len(cfg.Outbounds) != 2 {
		t.Fatalf("chain lost: %d", len(cfg.Outbounds))
	}
	stream := cfg.Outbounds[0]["streamSettings"].(map[string]any)
	if stream["sockopt"].(map[string]any)["dialerProxy"] != cfg.Outbounds[1]["tag"] {
		t.Fatal("exit bypasses relay")
	}
	if _, ok := cfg.Outbounds[1]["streamSettings"].(map[string]any)["sockopt"]; ok {
		t.Fatal("entry should connect directly")
	}
}
func TestChainRejectsPrivateLaterHop(t *testing.T) {
	base := "vless://11111111-1111-4111-8111-111111111111@8.8.8.8:443?security=reality&sni=example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	r, e := RunPath(context.Background(), []Hop{{Name: "entry", Link: base}, {Name: "exit", Link: strings.Replace(base, "8.8.8.8", "127.0.0.1", 1)}}, "cloudflare")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(r.Summary, "地址") || r.Steps[len(r.Steps)-1].Status != "failed" {
		t.Fatalf("private exit accepted: %+v", r)
	}
}
