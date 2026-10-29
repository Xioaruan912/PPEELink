package tracecore

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNextTraceJSONOfficialSchema(t *testing.T) {
	raw := `{"Hops":[[{"Success":true,"Address":{"IP":"1.1.1.1","Zone":""},"Hostname":"router.example","TTL":1,"RTT":1250000,"Geo":{"asnumber":"13335","country":"美国","prov":"加州","city":"洛杉矶","owner":"Cloudflare","isp":"Cloudflare","lat":34.05,"lng":-118.24}}],[{"Success":false,"Address":null,"TTL":2,"RTT":0}],[{"Success":false,"Address":null,"TTL":3,"RTT":0}]],"TraceMapUrl":""}`
	hops, err := ParseNextTrace(raw)
	if err != nil || len(hops) != 3 {
		t.Fatalf("hops=%+v err=%v", hops, err)
	}
	if hops[0].IP != "1.1.1.1" || hops[0].RTT == nil || *hops[0].RTT != 1.25 || hops[0].ASN != "13335" || hops[0].City != "洛杉矶" || hops[0].Hostname != "router.example" {
		t.Fatalf("hop %+v", hops[0])
	}
	if hops[1].Responded || hops[2].Responded || hops[1].TTL == hops[2].TTL {
		t.Fatal("unknown hops merged")
	}
	for _, raw := range []string{`{}`, `{"Hops":{}}`, `{"Hops":[[{"Success":true,"TTL":1,"Address":{"IP":"bad"}}]]}`, `{"Hops":`} {
		if _, err := ParseNextTrace(raw); err == nil {
			t.Fatalf("accepted invalid result %s", raw)
		}
	}
}
func TestTraceOptionsAndArguments(t *testing.T) {
	opts, err := NormalizeOptions("2001:db8::1", Options{Protocol: "tcp", Port: 443, Family: "6"})
	if err != nil {
		t.Fatal(err)
	}
	args, err := CommandArgs("nexttrace", "2001:db8::1", opts)
	want := []string{"--json", "--map", "--no-color", "--no-rdns", "--max-hops", "24", "--queries", "1", "--timeout", "1000", "--ipv6", "--tcp", "--port", "443", "2001:db8::1"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%q err=%v", args, err)
	}
	for _, o := range []Options{{Protocol: "exec"}, {Protocol: "tcp", Port: 65536}, {Family: "3"}, {Family: "4"}} {
		if _, err := NormalizeOptions("2001:db8::1", o); err == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
	if _, err := CommandArgs("tracepath", "example.com", Options{Protocol: "tcp", Port: 443, Family: "4"}); err == nil {
		t.Fatal("tracepath silently changed protocol")
	}
	if !LegacyCompatible(Options{}) || LegacyCompatible(Options{Protocol: "icmp", Family: "4"}) {
		t.Fatal("legacy compatibility incorrect")
	}
}

func TestNextTraceGeoFailureKeepsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nexttrace")
	script := `#!/bin/sh
case "$*" in
 *disable-geoip*) echo '{"Hops":[[{"Success":true,"TTL":1,"Address":{"IP":"127.0.0.1"},"RTT":1000000}]]}';;
 *) echo 'GeoAPI unavailable' >&2; exit 1;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	result := runNextTrace(context.Background(), path, []string{"--json", "--map", "127.0.0.1"})
	hops, err := ParseNextTrace(result.Output)
	if result.Error != "" || err != nil || len(hops) != 1 || hops[0].IP != "127.0.0.1" {
		t.Fatalf("%+v hops=%+v err=%v", result, hops, err)
	}
}
