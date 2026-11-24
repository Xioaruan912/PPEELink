package main

import (
	"strings"
	"testing"
)

func TestRewriteSystemdPanelURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "double_quoted",
			in:   `ExecStart=/usr/local/bin/ppeelink-agent agent -panel "https://old.example.com" -token-file /etc/ppeelink/token`,
			want: `ExecStart=/usr/local/bin/ppeelink-agent agent -panel "https://new.example.com" -token-file /etc/ppeelink/token`,
		},
		{
			name: "single_quoted",
			in:   `ExecStart=/usr/local/bin/ppeelink-agent agent -panel 'https://old.example.com' -token abc`,
			want: `ExecStart=/usr/local/bin/ppeelink-agent agent -panel 'https://new.example.com' -token abc`,
		},
		{
			name: "unquoted_space",
			in:   `ExecStart=/usr/local/bin/ppeelink-agent agent -panel https://old.example.com -token abc`,
			want: `ExecStart=/usr/local/bin/ppeelink-agent agent -panel https://new.example.com -token abc`,
		},
		{
			name: "equals_form",
			in:   `ExecStart=/usr/local/bin/ppeelink-agent agent -panel=https://old.example.com -token=abc`,
			want: `ExecStart=/usr/local/bin/ppeelink-agent agent -panel=https://new.example.com -token=abc`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := rewriteSystemdPanelURL(tc.in, "https://new.example.com")
			if !ok {
				t.Fatalf("expected match for %q", tc.in)
			}
			if got != tc.want {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
			if strings.Contains(got, "old.example.com") {
				t.Fatalf("old URL still present: %q", got)
			}
		})
	}
}
