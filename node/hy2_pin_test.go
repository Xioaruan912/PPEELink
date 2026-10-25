package node

import (
	"encoding/pem"
	"net/http/httptest"
	"ppeelink/certpin"
	"strings"
	"testing"
)

func TestHY2PinSurvivesSerializers(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	cert := server.Certificate()
	public := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	_, pin, e := certpin.Parse(public)
	if e != nil {
		t.Fatal(e)
	}
	h := HY2{Name: "pinned", Host: "127.0.0.1", Port: 24443, Password: "secret", Sni: cert.DNSNames[0], Insecure: 1, PinSHA256: pin, Certificate: public}
	link := EncodeHY2URL(h)
	parsed, e := DecodeHY2URL(link)
	if e != nil || parsed.Certificate != public || parsed.PinSHA256 != pin {
		t.Fatalf("identity lost: %v", e)
	}
	proxies := BuildClashProxies([]string{link}, nil, SqlConfig{})
	if len(proxies) != 1 || proxies[0].Fingerprint != pin || proxies[0].Skip_cert_verify {
		t.Fatal("Mihomo fingerprint lost or bypass enabled")
	}
	out, _, e := singboxOutbound(link)
	if e != nil {
		t.Fatal(e)
	}
	tls := out["tls"].(map[string]interface{})
	if tls["insecure"] != false || tls["certificate"].([]string)[0] != public {
		t.Fatal("sing-box trust lost")
	}
	probe, _, e := buildOutboundConfig(link)
	if e != nil || !strings.Contains(probe, "certificate") || strings.Contains(probe, `"insecure":true`) {
		t.Fatalf("probe bypassed verification: %v", e)
	}
	h.PinSHA256 = strings.Repeat("0", 64)
	if _, e = DecodeHY2URL(EncodeHY2URL(h)); e == nil {
		t.Fatal("wrong fingerprint accepted")
	}
	h.Certificate = ""
	if _, _, e = singboxOutbound(EncodeHY2URL(h)); e == nil {
		t.Fatal("pin-only URI downgraded to insecure sing-box")
	}
	h.PinSHA256 = ""
	h.Insecure = 0
	out, _, e = singboxOutbound(EncodeHY2URL(h))
	if e != nil || out["tls"].(map[string]interface{})["insecure"] != false {
		t.Fatal("ordinary certificate verification disabled")
	}
}
