package node

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestAppRulesGeneratedBeforeCatchall(t *testing.T) {
	var cfg SqlConfig
	json.Unmarshal([]byte(`{"appRules":[{"platform":"desktop","app":"chrome.exe","target":"Japan"},{"platform":"android","app":"org.telegram.messenger","target":"DIRECT"}]}`), &cfg)
	out, err := ApplyApplicationRules("clash", []byte("proxies:\n  - name: Japan\n    type: socks5\nrules:\n  - MATCH,DIRECT\n"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	yaml.Unmarshal(out, &doc)
	r := doc["rules"].([]any)
	if r[0] != "PROCESS-NAME,chrome.exe,Japan" || r[1] != "PROCESS-NAME,org.telegram.messenger,DIRECT" {
		t.Fatal(string(out))
	}
	if doc["find-process-mode"] != "always" || doc["mode"] != "rule" {
		t.Fatal(string(out))
	}
}
func TestAppRulesRejectInjectionAndMissingTargets(t *testing.T) {
	for _, app := range []string{"chrome.exe,DIRECT", "bad\nMATCH", "../app"} {
		var cfg SqlConfig
		raw, _ := json.Marshal(map[string]any{"appRules": []any{map[string]string{"platform": "desktop", "app": app, "target": "DIRECT"}}})
		json.Unmarshal(raw, &cfg)
		if _, err := ApplyApplicationRules("clash", []byte("rules: []"), cfg); err == nil {
			t.Fatal(app)
		}
	}
	var cfg SqlConfig
	json.Unmarshal([]byte(`{"appRules":[{"platform":"desktop","app":"chrome.exe","target":"gone"}]}`), &cfg)
	if _, err := ApplyApplicationRules("singbox", []byte(`{"outbounds":[{"tag":"proxy"}],"route":{"final":"proxy"}}`), cfg); err == nil {
		t.Fatal("missing target accepted")
	}
}
func TestAndroidSingboxUsesPackageAndRejectAction(t *testing.T) {
	var cfg SqlConfig
	json.Unmarshal([]byte(`{"appRules":[{"platform":"android","app":"com.example.app","target":"REJECT"}]}`), &cfg)
	out, err := ApplyApplicationRules("singbox", []byte(`{"outbounds":[{"tag":"proxy"}],"route":{"final":"proxy"}}`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"package_name"`) || !strings.Contains(string(out), `"action": "reject"`) {
		t.Fatal(string(out))
	}
}
