package xraycore

import (
	"encoding/json"
	"testing"
)

func TestHysteriaInboundConfiguration(t *testing.T) {
	raw := `{"tag":"hy","template":"hysteria2","port":20443,"sni":"proxy.example.com","tlsCertFile":"/etc/ssl/fullchain.pem","tlsKeyFile":"/etc/ssl/privkey.pem","users":[{"email":"u1","uuid":"test-password"}]}`
	var in InboundSpec
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}
	c, err := buildInbound(in)
	if err != nil {
		t.Fatal(err)
	}
	if c["protocol"] != "hysteria" {
		t.Fatal(c)
	}
	stream := c["streamSettings"].(map[string]any)
	if stream["network"] != "hysteria" || stream["security"] != "tls" {
		t.Fatal(stream)
	}
	settings := c["settings"].(map[string]any)
	if settings["version"] != 2 {
		t.Fatal(settings)
	}
	user := settings["clients"].([]any)[0].(map[string]any)
	if user["email"] != "u1" || user["auth"] != "test-password" {
		t.Fatal(user)
	}
}
func TestHysteriaRejectsMissingCertificates(t *testing.T) {
	var in InboundSpec
	json.Unmarshal([]byte(`{"tag":"hy","template":"hysteria2","port":20443,"sni":"proxy.example.com"}`), &in)
	if _, err := buildInbound(in); err == nil {
		t.Fatal("missing TLS cert accepted")
	}
}
