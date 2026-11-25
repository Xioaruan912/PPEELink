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

func TestMultiProcessAppBundleExpansionClashAndSingbox(t *testing.T) {
	rawCfg := `{
		"appRules": [{
			"platform": "all",
			"preset": "chatgpt",
			"label": "ChatGPT / Codex",
			"processes": ["ChatGPT.exe", "ChatGPT", "codex.exe", "codex"],
			"packages": ["com.openai.chatgpt"],
			"target": "ChatGPT"
		}]
	}`
	var cfg SqlConfig
	if err := json.Unmarshal([]byte(rawCfg), &cfg); err != nil {
		t.Fatal(err)
	}
	clashTpl := "proxy-groups:\n  - name: ChatGPT\n    type: select\n    proxies: [DIRECT]\nrules:\n  - MATCH,DIRECT\n"
	out, err := ApplyApplicationRules("clash", []byte(clashTpl), cfg)
	if err != nil {
		t.Fatalf("clash multi-process bundle failed: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	rules := doc["rules"].([]any)
	want := []string{
		"PROCESS-NAME,ChatGPT.exe,ChatGPT",
		"PROCESS-NAME,ChatGPT,ChatGPT",
		"PROCESS-NAME,codex.exe,ChatGPT",
		"PROCESS-NAME,codex,ChatGPT",
		"PROCESS-NAME,com.openai.chatgpt,ChatGPT",
	}
	for i, w := range want {
		if rules[i] != w {
			t.Fatalf("rule[%d] = %v, want %s", i, rules[i], w)
		}
	}

	// Sing-box should emit both process_name array and package_name array cleanly.
	cfg.AppRules[0].Target = "PROXY"
	sbOut, err := ApplyApplicationRules("singbox", []byte(`{"outbounds":[{"tag":"proxy"}],"route":{"final":"proxy"}}`), cfg)
	if err != nil {
		t.Fatalf("singbox multi-process bundle failed: %v", err)
	}
	sbText := string(sbOut)
	if !strings.Contains(sbText, `"codex.exe"`) || !strings.Contains(sbText, `"com.openai.chatgpt"`) || !strings.Contains(sbText, `"process_name"`) || !strings.Contains(sbText, `"package_name"`) {
		t.Fatalf("unexpected singbox output: %s", sbText)
	}
}
