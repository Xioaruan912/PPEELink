package api

import (
	"encoding/json"
	"testing"
)

func TestEnsureDefaultSubUDP(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantUDP bool
	}{
		{"empty", "", true},
		{"omitted", `{"clash":"./template/clash.yaml"}`, true},
		{"explicit_false", `{"clash":"./template/clash.yaml","udp":false}`, false},
		{"explicit_true", `{"clash":"./template/clash.yaml","udp":true}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ensureDefaultSubUDP(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("unmarshal %q: %v", out, err)
			}
			if got["udp"] != tc.wantUDP {
				t.Fatalf("udp=%v, want %v (raw=%s)", got["udp"], tc.wantUDP, out)
			}
		})
	}
}
