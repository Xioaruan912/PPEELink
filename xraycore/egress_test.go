package xraycore

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFixedEgressConfig(t *testing.T) {
	s := websiteTestSpec()
	s.WebsiteTraffic = nil
	raw := `{"address":"exit.example.com","port":24443,"uuid":"11111111-1111-4111-8111-111111111111","sni":"www.example.com","publicKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","transport":"raw","flow":"xtls-rprx-vision"}`
	data, _ := json.Marshal(s)
	var doc map[string]any
	json.Unmarshal(data, &doc)
	var exit map[string]any
	json.Unmarshal([]byte(raw), &exit)
	doc["inbounds"].([]any)[0].(map[string]any)["egress"] = exit
	data, _ = json.Marshal(doc)
	json.Unmarshal(data, &s)
	out, err := BuildConfig(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"exit-in-7"`) {
		t.Fatal("fixed egress route absent")
	}
	s.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
	c := websiteTestCollector(t, s)
	out, err = BuildConfigWithWebsiteCollector(s, c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"bridge-in-7"`) || !strings.Contains(string(out), `"website-0"`) {
		t.Fatal("metering path bypassed")
	}
}
