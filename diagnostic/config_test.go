package diagnostic

import (
	"strings"
	"testing"
)

func TestOutbound(t *testing.T) {
	base := "vless://11111111-1111-4111-8111-111111111111@8.8.8.8:443?security=reality&sni=example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&sid=abcd"
	for _, transport := range []string{"tcp", "xhttp", "ws", "grpc", "httpupgrade"} {
		p, e := parse(base + "&type=" + transport)
		if e != nil || p.protocol != "Reality" {
			t.Fatalf("%s %v", transport, e)
		}
	}
	if _, e := parse(base + "&type=invalid"); e == nil {
		t.Fatal("unsupported transport ignored")
	}
	if _, e := parse("hy2://secret@8.8.8.8:443?sni=example.com&insecure=1"); e == nil {
		t.Fatal("unverified TLS accepted")
	}
	if _, e := parse("hy2://secret@8.8.8.8:443?sni=example.com&pinSHA256=" + strings.Repeat("a", 64) + "&obfs=salamander&obfs-password=abc"); e != nil {
		t.Fatal(e)
	}
}
func TestPrivateAddresses(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::ffff:127.0.0.1"} {
		if publicIP(ip) {
			t.Fatalf("accepted %s", ip)
		}
	}
}

func TestHTTP403NotAuthentication(t *testing.T) {
	b := &safeLog{}
	b.Write([]byte("HTTP 403"))
	if strings.Contains(b.hint(), "认证被拒绝") {
		t.Fatal("HTTP403 misclassified as auth failure")
	}
}
