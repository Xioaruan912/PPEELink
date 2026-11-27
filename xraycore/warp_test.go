package xraycore

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildConfigWithWarpOutbound(t *testing.T) {
	spec := Spec{
		APIPort: 62789,
		Enabled: true,
		Warp: &WarpSpec{
			Enabled: true,
			Domains: []string{"openai", "claude", "gemini", "example.com"},
			Profile: &WarpProfile{
				PrivateKey: "test-priv-key",
				PeerPubKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
				Endpoint:   "engage.cloudflareclient.com:2408",
				AddressV4:  "172.16.0.2",
				AddressV6:  "2606:4700:110:8a36:df92:102a:9602:fa18",
				Reserved:   []int{1, 2, 3},
			},
		},
	}
	raw, err := BuildConfig(spec)
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, `"tag": "warp-out"`) || !strings.Contains(text, `"protocol": "wireguard"`) {
		t.Fatalf("expected wireguard warp-out outbound in config:\n%s", text)
	}
	if !strings.Contains(text, `"domain:openai.com"`) || !strings.Contains(text, `"domain:claude.ai"`) || !strings.Contains(text, `"domain:gemini.google.com"`) || !strings.Contains(text, `"domain:example.com"`) {
		t.Fatalf("expected expanded warp domains in routing rules:\n%s", text)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
