package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeClashMovesGlobalFingerprintToProxies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.yaml")
	tmpl := "global-client-fingerprint: chrome\nproxies: ~\nproxy-groups:\n  - name: sel\n    type: select\n    proxies: []\nrules:\n  - MATCH,sel\n"
	if err := os.WriteFile(path, []byte(tmpl), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := DecodeClash([]Proxy{
		{Name: "tls", Type: "vless", Server: "a.example.com", Port: 443, Tls: true},
		{Name: "own", Type: "trojan", Server: "b.example.com", Port: 443, Client_fingerprint: "firefox"},
		{Name: "plain", Type: "ss", Server: "c.example.com", Port: 8388},
	}, path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "global-client-fingerprint") {
		t.Fatal("removed mihomo key must not be emitted")
	}
	if strings.Count(text, "client-fingerprint: chrome") != 1 || !strings.Contains(text, "client-fingerprint: firefox") {
		t.Fatalf("fingerprint must move onto TLS proxies without overriding their own:\n%s", text)
	}
}
