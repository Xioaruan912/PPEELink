package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoonPreservesPinWithoutGeneratedClientPolicies(t *testing.T) {
	f := filepath.Join(t.TempDir(), "loon.conf")
	os.WriteFile(f, []byte("[Proxy]\nold = trojan,old.invalid,443,pass\n[Proxy Group]\nGlobal = select,DIRECT\n[Rule]\nFINAL,Global\n"), 0600)
	a := "vless://11111111-1111-4111-8111-111111111111@1.1.1.1:443?security=reality&sni=example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#HK"
	b := "hy2://secret@8.8.8.8:443?sni=example.com&pinSHA256=" + strings.Repeat("a", 64) + "#JP"
	out, e := EncodeLoonWithFlags([]string{a, b}, nil, SqlConfig{Loon: f, Udp: true})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out, "[Proxy Chain]") || strings.Contains(out, "中转线路") {
		t.Fatalf("retired client policy generated: %s", out)
	}
	if !strings.Contains(out, "tls-cert-sha256="+strings.Repeat("a", 64)) || strings.Contains(out, "skip-cert-verify=true") {
		t.Fatal("pin lost")
	}
	if strings.Contains(out, "old.invalid") {
		t.Fatal("stale proxy definitions retained")
	}
	if !strings.Contains(out, "Global = select,DIRECT") || !strings.Contains(out, "JP = Hysteria2") {
		t.Fatal("template policies or node definition changed")
	}
}
func TestDecodeLoonProxyAtEOF(t *testing.T) {
	f := filepath.Join(t.TempDir(), "config")
	os.WriteFile(f, []byte("[General]\nip-mode=v4-only\n[Proxy]\nstale = test\n"), 0600)
	out, e := DecodeLoon([]string{"fresh = test"}, f)
	if e != nil || strings.Contains(out, "stale") || !strings.Contains(out, "fresh") {
		t.Fatalf("%s %v", out, e)
	}
}
