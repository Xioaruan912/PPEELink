package api

import "testing"

func TestParseStructuredLogs(t *testing.T) {
	raw := `2026-11-24T11:40:20+0000 vps1 ppeelink-xray[1234]: 2026/11/24 19:40:20.123 [Warning] [99] proxy/vless/inbound: reality handshake failed: invalid shortId
2026-11-24T11:40:21+0000 vps1 ppeelink-xray[1234]: 2026/11/24 19:40:21.456 [Warning] [100] proxy/vless/inbound: reality handshake failed: invalid shortId
2026-11-24T11:40:22+0000 vps1 ppeelink-xray[1234]: 2026/11/24 19:40:22 [Error] failed to listen on tcp:0.0.0.0:443 > address already in use`

	entries, summary := parseStructuredLogs(raw, "xray")
	if len(entries) != 2 {
		t.Fatalf("expected 2 deduplicated entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Count != 2 || entries[0].Module != "Reality" || entries[0].Hint == "" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Level != "error" || entries[1].Hint == "" {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}
	if summary.Errors != 1 || summary.Warnings != 2 || len(summary.Findings) != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
