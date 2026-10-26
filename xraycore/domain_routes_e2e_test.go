package xraycore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/net/dns/dnsmessage"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func domainTestDNS(t *testing.T) {
	t.Helper()
	ln, e := net.ListenPacket("udp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	original := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", ln.LocalAddr().String())
	}}
	t.Cleanup(func() { net.DefaultResolver = original; ln.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, a, e := ln.ReadFrom(buf)
			if e != nil {
				return
			}
			var msg dnsmessage.Message
			if msg.Unpack(buf[:n]) != nil {
				continue
			}
			msg.Header.Response = true
			msg.Header.RecursionAvailable = true
			msg.Answers = nil
			for _, q := range msg.Questions {
				if q.Type == dnsmessage.TypeA {
					msg.Answers = append(msg.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 1}, Body: &dnsmessage.AResource{A: [4]byte{127, 0, 0, 1}}})
				}
			}
			raw, e := msg.Pack()
			if e == nil {
				ln.WriteTo(raw, a)
			}
		}
	}()
}
func TestFeatureDomainRoutingRealCore(t *testing.T) {
	if os.Getenv("XRAY_FEATURE_BINARY") == "" {
		t.Skip("requires real core")
	}
	domainTestDNS(t)
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "direct") }))
	defer direct.Close()
	exit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "exit") }))
	defer exit.Close()
	udpEcho := func(prefix string) *net.UDPConn {
		l, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { l.Close() })
		go func() {
			buf := make([]byte, 64)
			for {
				n, a, e := l.ReadFromUDP(buf)
				if e != nil {
					return
				}
				l.WriteToUDP(append([]byte(prefix), buf[:n]...), a)
			}
		}()
		return l
	}
	directUDP, exitUDP := udpEcho("D"), udpEcho("E")
	egress := &EgressSpec{Address: "8.8.8.8", Port: 24443, UUID: "11111111-1111-4111-8111-111111111111", SNI: "www.example.com", PublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Transport: "raw", Flow: "xtls-rprx-vision"}
	for _, metered := range []bool{false, true} {
		t.Run(fmt.Sprint("metered-", metered), func(t *testing.T) {
			s := websiteTestSpec()
			s.BlockPrivate = false
			s.WebsiteTraffic = nil
			s.APIPort = featurePort(t)
			b, err := newEgressBridge()
			if err != nil {
				t.Fatal(err)
			}
			s.Inbounds = s.Inbounds[:1]
			s.Inbounds[0].Port = b.port
			s.Inbounds[0].DomainRoutes = []DomainRouteSpec{{Domain: "api.example.com", Match: "exact"}, {Domain: "example.com", Match: "suffix", Egress: egress}, {Domain: "dead.example.net", Match: "exact", Egress: egress}}
			var collector *WebsiteCollector
			if metered {
				s.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
				collector = websiteTestCollector(t, s)
			}
			raw, err := BuildConfigWithWebsiteCollector(s, collector)
			if err != nil {
				t.Fatal(err)
			}
			var cfg map[string]any
			json.Unmarshal(raw, &cfg)
			for i, item := range cfg["inbounds"].([]any) {
				m := item.(map[string]any)
				if m["tag"] == "in-7" {
					sock := featureSocks(b)
					sock["tag"] = "in-7"
					sock["sniffing"] = m["sniffing"]
					cfg["inbounds"].([]any)[i] = sock
				}
			}
			dead := net.JoinHostPort("127.0.0.1", strconv.Itoa(featurePort(t)))
			for _, item := range cfg["outbounds"].([]any) {
				m := item.(map[string]any)
				tag := m["tag"].(string)
				if strings.HasPrefix(tag, "route-exit-") || strings.HasPrefix(tag, "exit-route-") {
					redirect := exit.Listener.Addr().String()
					if strings.HasSuffix(tag, "-2") {
						redirect = dead
					}
					m["protocol"] = "freedom"
					m["settings"] = map[string]any{"redirect": redirect}
					delete(m, "streamSettings")
				}
			}
			featureCore(t, cfg)
			proxy, _ := url.Parse(fmt.Sprintf("socks5://%s:%s@127.0.0.1:%d", b.user, b.password, b.port))
			for _, tc := range []struct{ host, want string }{{"api.example.com", "direct"}, {"x.example.com", "exit"}, {"badexample.com", "direct"}, {"unknown.example.net", "direct"}, {"dead.example.net", ""}} {
				tr := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
				client := http.Client{Transport: tr, Timeout: time.Second}
				req, _ := http.NewRequest("GET", direct.URL, nil)
				req.Host = tc.host
				resp, err := client.Do(req)
				if tc.want == "" {
					if err == nil {
						resp.Body.Close()
						t.Fatal("dead exit fell back to direct")
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s: %v", tc.host, err)
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				tr.CloseIdleConnections()
				if string(body) != tc.want {
					t.Fatalf("%s: got %s want %s", tc.host, body, tc.want)
				}
			}
			if metered {
				records := collector.Drain()
				if len(records) == 0 {
					t.Fatal("missing metering")
				}
				for _, r := range records {
					if r.InboundTag != "in-7" {
						t.Fatalf("wrong attribution %s", r.InboundTag)
					}
				}
			}
			// A second isolated config covers UDP hostname routing with deterministic DNS.
			ub, err := newEgressBridge()
			if err != nil {
				t.Fatal(err)
			}
			s.Inbounds[0].Port = ub.port
			s.APIPort = featurePort(t)
			var uc *WebsiteCollector
			if metered {
				uc = websiteTestCollector(t, s)
			}
			raw, err = BuildConfigWithWebsiteCollector(s, uc)
			if err != nil {
				t.Fatal(err)
			}
			json.Unmarshal(raw, &cfg)
			cfg["dns"] = map[string]any{"hosts": map[string]any{"api.example.com": "127.0.0.1", "x.example.com": "127.0.0.1"}}
			for i, item := range cfg["inbounds"].([]any) {
				m := item.(map[string]any)
				if m["tag"] == "in-7" {
					sock := featureSocks(ub)
					sock["tag"] = "in-7"
					cfg["inbounds"].([]any)[i] = sock
				}
			}
			for _, item := range cfg["outbounds"].([]any) {
				m := item.(map[string]any)
				tag := m["tag"].(string)
				if strings.HasPrefix(tag, "route-exit-") || strings.HasPrefix(tag, "exit-route-") {
					m["protocol"] = "freedom"
					m["settings"] = map[string]any{"redirect": exitUDP.LocalAddr().String()}
					delete(m, "streamSettings")
				} else if m["protocol"] == "freedom" {
					m["settings"] = map[string]any{"domainStrategy": "UseIPv4"}
				}
			}
			featureCore(t, cfg)
			for _, tc := range []struct{ host, want string }{{"api.example.com", "Dping"}, {"x.example.com", "Eping"}} {
				c, err := ub.dial(context.Background(), "udp", net.JoinHostPort(tc.host, strconv.Itoa(directUDP.LocalAddr().(*net.UDPAddr).Port)))
				if err != nil {
					t.Fatal(err)
				}
				c.SetDeadline(time.Now().Add(3 * time.Second))
				// Production bridges intentionally receive prevalidated IPs. This test
				// client sends a SOCKS domain explicitly to exercise UDP routing.
				udp := c.(*egressUDPConn)
				dp := directUDP.LocalAddr().(*net.UDPAddr).Port
				udp.header = append([]byte{0, 0, 0, 3, byte(len(tc.host))}, []byte(tc.host)...)
				udp.header = append(udp.header, byte(dp>>8), byte(dp))
				c.Write([]byte("ping"))
				packet := make([]byte, 128)
				size, err := udp.UDPConn.Read(packet)
				buf := []byte{}
				n := 0
				if err == nil && size >= 4 {
					reader := bytes.NewReader(packet[4:size])
					_, _, err = readWebsiteAddress(reader, packet[3])
					buf = packet[size-reader.Len() : size]
					n = len(buf)
				}
				c.Close()
				if err != nil || string(buf[:n]) != tc.want {
					t.Fatalf("UDP %s: %q %v", tc.host, buf[:n], err)
				}
			}
			if metered {
				for _, r := range uc.Drain() {
					if r.InboundTag != "in-7" {
						t.Fatalf("UDP wrong attribution %s", r.InboundTag)
					}
				}
			}
		})
	}
}
