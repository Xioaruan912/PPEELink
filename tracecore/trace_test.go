package tracecore

import (
	"context"
	"strings"
	"testing"
)

func TestValidateTarget(t *testing.T) {
	for _, s := range []string{"example.com", "EXAMPLE.com.", "1.1.1.1", "2001:4860:4860::8888", "localhost"} {
		if _, err := ValidateTarget(s); err != nil {
			t.Errorf("valid %q: %v", s, err)
		}
	}
	for _, s := range []string{"", "https://example.com/a", "example.com:80", "-n", "x; touch /tmp/x", "x\n-n", "::", "224.0.0.1", "bad_label.com", "a..com", strings.Repeat("a", 254)} {
		if _, err := ValidateTarget(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func TestParseTraceFormats(t *testing.T) {
	out := "traceroute to x (1.1.1.1), 24 hops max\n 1  192.168.1.1  1.123 ms\n 2  *\n 3  2001:4860::1  12.4 ms !H\n 4: 1.1.1.1 24.50ms reached\n"
	hops := Parse(out)
	if len(hops) != 4 {
		t.Fatalf("%+v", hops)
	}
	if hops[0].IP != "192.168.1.1" || hops[0].RTT == nil || *hops[0].RTT != 1.123 || hops[1].Responded || hops[2].IP != "2001:4860::1" || *hops[3].RTT != 24.5 {
		t.Fatalf("unexpected %+v", hops)
	}
	if len(Parse("999 1.1.1.1 3ms\n unrelated output")) != 0 {
		t.Fatal("accepted invalid hop")
	}
}
func TestBoundedTraceOutput(t *testing.T) {
	out := &boundedOutput{}
	p := []byte(strings.Repeat("x", MaxOutput+100))
	n, err := out.Write(p)
	if err != nil || n != len(p) || len(out.String()) != MaxOutput {
		t.Fatal("output not bounded")
	}
	if result := Run(context.Background(), ";bad"); result.Error == "" || result.Output != "" {
		t.Fatal("invalid target executed")
	}
}
