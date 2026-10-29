package xraycore

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func hy2TestEgress(t *testing.T, extra string) *EgressSpec {
	t.Helper()
	var e EgressSpec
	if err := json.Unmarshal([]byte(`{"protocol":"hysteria2","address":"exit.example.com","port":24443,"password":"test-secret","sni":"exit.example.com"`+extra+`}`), &e); err != nil {
		t.Fatal(err)
	}
	return &e
}
func TestHY2EgressVerifiedTLSAndTransport(t *testing.T) {
	e := hy2TestEgress(t, `,"pinSHA256":"`+strings.Repeat("AB:", 31)+`AB","obfs":"salamander","obfsPassword":"obfs-secret","alpn":["h3"]`)
	if err := e.Validate(); err != nil {
		t.Fatalf("secure HY2 exit rejected: %v", err)
	}
	out := e.outbound("exit")
	if out["protocol"] != "hysteria" {
		t.Fatalf("wrong protocol: %v", out)
	}
	stream := out["streamSettings"].(map[string]any)
	tls := stream["tlsSettings"].(map[string]any)
	if tls["serverName"] != "exit.example.com" || tls["pinnedPeerCertSha256"] != strings.Repeat("ab", 32) || tls["allowInsecure"] == true {
		t.Fatalf("TLS verification lost: %v", tls)
	}
	if stream["network"] != "hysteria" || stream["security"] != "tls" || stream["hysteriaSettings"].(map[string]any)["auth"] != "test-secret" {
		t.Fatalf("wrong HY2 stream: %v", stream)
	}
	mask := stream["finalmask"].(map[string]any)["udp"].([]any)[0].(map[string]any)
	if mask["type"] != "salamander" || mask["settings"].(map[string]any)["password"] != "obfs-secret" {
		t.Fatalf("obfs lost: %v", mask)
	}
}
func TestHY2EgressDefaultsToVerifiedPublicTLS(t *testing.T) {
	e := hy2TestEgress(t, "")
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	tls := e.outbound("exit")["streamSettings"].(map[string]any)["tlsSettings"].(map[string]any)
	if tls["allowInsecure"] == true || !reflect.DeepEqual(tls["alpn"], []string{"h3"}) {
		t.Fatal(tls)
	}
}
func TestHY2EgressRejectsInvalidSettings(t *testing.T) {
	for _, extra := range []string{`,"pinSHA256":"bad"`, `,"password":""`, `,"sni":"bad/name"`, `,"obfs":"unknown","obfsPassword":"secret"`, `,"obfs":"salamander"`, `,"obfsPassword":"orphan"`, `,"alpn":[""]`, `,"address":"127.0.0.1"`, `,"address":"localhost"`, `,"address":"host:443"`, `,"protocol":"unknown"`} {
		t.Run(extra, func(t *testing.T) {
			if err := hy2TestEgress(t, extra).Validate(); err == nil {
				t.Fatal("invalid exit accepted")
			}
		})
	}
}

func TestEgressRejectsNonPublicAddressSpellings(t *testing.T) {
	for _, address := range []string{"127.0.0.1.", "192.168.1.1.", "0.0.0.0.", "100.64.0.1.", "::ffff:127.0.0.1", "localhost.", "host.local."} {
		t.Run(address, func(t *testing.T) {
			e := hy2TestEgress(t, "")
			e.Address = address
			if err := e.Validate(); err == nil {
				t.Fatal("non-public address accepted")
			}
		})
	}
}

func TestRealityEgressRejectsHY2OnlySettings(t *testing.T) {
	for _, extra := range []string{`"password":"secret"`, `"pinSHA256":"` + strings.Repeat("ab", 32) + `"`, `"obfs":"salamander","obfsPassword":"secret"`, `"alpn":["h3"]`} {
		t.Run(extra, func(t *testing.T) {
			var e EgressSpec
			if err := json.Unmarshal([]byte(`{"address":"exit.example.com","port":443,"uuid":"11111111-1111-4111-8111-111111111111","sni":"www.example.com","publicKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","transport":"raw",`+extra+`}`), &e); err != nil {
				t.Fatal(err)
			}
			if err := e.Validate(); err == nil {
				t.Fatal("HY2 settings silently ignored by Reality exit")
			}
		})
	}
}
func TestSwitchingToHY2EgressPreservesEntryConfiguration(t *testing.T) {
	s := websiteTestSpec()
	s.WebsiteTraffic = nil
	before, err := BuildConfig(s)
	if err != nil {
		t.Fatal(err)
	}
	s.Inbounds[0].Egress = hy2TestEgress(t, "")
	after, err := BuildConfig(s)
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	json.Unmarshal(before, &a)
	json.Unmarshal(after, &b)
	if !reflect.DeepEqual(a["inbounds"], b["inbounds"]) {
		t.Fatal("changing exit changed stable entry credentials or listener")
	}
}
