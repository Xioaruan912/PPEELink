package api

import "testing"

func TestParseSubscriptionUserinfo(t *testing.T) {
	up, down, total, expire, ok := parseSubscriptionUserinfo("upload=1024; download=2048;total=10737418240; expire=1767225600")
	if !ok || up != 1024 || down != 2048 || total != 10737418240 || expire == nil || expire.Unix() != 1767225600 {
		t.Fatalf("unexpected parse: %d %d %d %v %v", up, down, total, expire, ok)
	}
	if _, _, _, _, ok := parseSubscriptionUserinfo(""); ok {
		t.Fatal("empty header must not be ok")
	}
	_, _, total, expire, ok = parseSubscriptionUserinfo("upload=0; download=0; total=1.5e10; expire=0")
	if !ok || total != 15000000000 || expire != nil {
		t.Fatalf("float total / zero expire: %d %v %v", total, expire, ok)
	}
}
