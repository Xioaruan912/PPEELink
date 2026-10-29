package xraycore

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"ppeelink/certpin"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFeatureHY2GatewayRealCore(t *testing.T) {
	if os.Getenv("XRAY_FEATURE_BINARY") == "" {
		t.Skip("requires real core")
	}
	domainTestDNS(t)
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); io.WriteString(w, "through-hy2-exit") }))
	defer target.Close()
	// Only the upstream redirects to this backend. A gateway falling back to
	// direct routing reaches a different response, even though both are local.
	var bypasses atomic.Int32
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { bypasses.Add(1); io.WriteString(w, "bypassed-hy2") }))
	defer direct.Close()
	udp, err := net.ListenPacket("udp4", target.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	var udpHits atomic.Int32
	go func() {
		buf := make([]byte, 1024)
		for {
			n, peer, err := udp.ReadFrom(buf)
			if err != nil {
				return
			}
			udpHits.Add(1)
			udp.WriteTo(append([]byte("through-hy2:"), buf[:n]...), peer)
		}
	}()
	decoy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "decoy") }))
	decoy.Config.ErrorLog = log.New(io.Discard, "", 0)
	decoy.EnableHTTP2 = true
	decoy.TLS = &tls.Config{MinVersion: tls.VersionTLS13}
	decoy.StartTLS()
	defer decoy.Close()
	dir := t.TempDir()
	cert, key, _, err := ensureAutoTLS(dir, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	publicPEM, err := os.ReadFile(cert)
	if err != nil {
		t.Fatal(err)
	}
	_, pin, err := certpin.Parse(string(publicPEM))
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	expired.NotBefore, expired.NotAfter = time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)
	der, err := x509.CreateCertificate(rand.Reader, expired, expired, expired.PublicKey, pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	expiredFile := filepath.Join(dir, "expired.pem")
	if err := os.WriteFile(expiredFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	upstreamPort := featurePort(t)
	exit := InboundSpec{Tag: "hy-exit", Template: TemplateHysteria2, Port: upstreamPort, SNI: AutoTLSServerName, TLSCertFile: cert, TLSKeyFile: key, Users: []UserSpec{{Email: "hy-user", UUID: "hy-secret"}}}
	obfsExit := exit
	obfsExit.Tag = "hy-obfs"
	obfsExit.Port = featurePort(t)
	expiredExit := exit
	expiredExit.Tag, expiredExit.Port, expiredExit.TLSCertFile = "hy-expired", featurePort(t), expiredFile
	raw, err := BuildConfig(Spec{APIPort: featurePort(t), Inbounds: []InboundSpec{exit, obfsExit, expiredExit}})
	if err != nil {
		t.Fatal(err)
	}
	var upstream map[string]any
	json.Unmarshal(raw, &upstream)
	for _, v := range upstream["inbounds"].([]any) {
		item := v.(map[string]any)
		if item["tag"] == "hy-obfs" {
			item["streamSettings"].(map[string]any)["finalmask"] = map[string]any{"udp": []any{map[string]any{"type": "salamander", "settings": map[string]any{"password": "obfs-secret"}}}}
		}
	}
	upstream["outbounds"] = append(upstream["outbounds"].([]any), map[string]any{"tag": "selected-exit", "protocol": "freedom", "settings": map[string]any{"redirect": target.Listener.Addr().String()}})
	routing := upstream["routing"].(map[string]any)
	routing["rules"] = append(routing["rules"].([]any), map[string]any{"inboundTag": []string{"hy-exit", "hy-obfs", "hy-expired"}, "outboundTag": "selected-exit"})
	featureCore(t, upstream)
	priv, pub, err := GenerateRealityKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, metered := range []bool{false, true} {
		for _, tc := range []struct {
			name, password, pin, sni string
			trusted, valid           bool
		}{
			{"correct", "hy-secret", pin, AutoTLSServerName, false, true},
			{"salamander", "hy-secret", pin, AutoTLSServerName, false, true},
			{"wrong-password", "wrong", pin, AutoTLSServerName, false, false},
			{"wrong-pin", "hy-secret", strings.Repeat("aa", 32), AutoTLSServerName, false, false},
			{"no-pin-untrusted-certificate", "hy-secret", "", AutoTLSServerName, false, false},
			{"ca-verified", "hy-secret", "", AutoTLSServerName, true, true},
			{"ca-wrong-server-name", "hy-secret", "", "wrong.example.com", true, false},
			{"ca-expired-certificate", "hy-secret", "", AutoTLSServerName, true, false},
			{"domain-route-correct", "hy-secret", pin, AutoTLSServerName, false, true},
			{"domain-route-wrong-pin", "hy-secret", strings.Repeat("aa", 32), AutoTLSServerName, false, false},
		} {
			t.Run(strconv.FormatBool(metered)+"/"+tc.name, func(t *testing.T) {
				if tc.trusted {
					// A fixture CA exercises normal PKI without changing emitted TLS settings.
					t.Setenv("SSL_CERT_FILE", cert)
					t.Setenv("SSL_CERT_DIR", dir)
				}
				entry := InboundSpec{Tag: "stable-entry", Template: TemplateRealityVision, Port: featurePort(t), Target: decoy.Listener.Addr().String(), SNI: "example.com", PrivateKey: priv, ShortID: "a1b2", Users: []UserSpec{{Email: "stable-user", UUID: "11111111-1111-4111-8111-111111111111"}}, Egress: &EgressSpec{Protocol: "hysteria2", Address: "8.8.8.8", Port: upstreamPort, Password: tc.password, SNI: tc.sni, PinSHA256: tc.pin}}
				if tc.name == "ca-expired-certificate" {
					entry.Egress.Port = expiredExit.Port
				}
				if tc.name == "salamander" {
					entry.Egress.Port = obfsExit.Port
					entry.Egress.Obfs = "salamander"
					entry.Egress.ObfsPassword = "obfs-secret"
				}
				if strings.HasPrefix(tc.name, "domain-route-") {
					entry.DomainRoutes = []DomainRouteSpec{{Domain: "gateway-test.example.com", Match: "exact", Egress: entry.Egress}}
					entry.Egress = nil
				}
				spec := Spec{APIPort: featurePort(t), Inbounds: []InboundSpec{entry}}
				var collector *WebsiteCollector
				if metered {
					spec.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
					collector = websiteTestCollector(t, spec)
				}
				raw, err := BuildConfigWithWebsiteCollector(spec, collector)
				if err != nil {
					t.Fatal(err)
				}
				var cfg map[string]any
				json.Unmarshal(raw, &cfg)
				// The fixture maps the validated public endpoint to its isolated local exit.
				for _, v := range cfg["outbounds"].([]any) {
					out := v.(map[string]any)
					if out["protocol"] == "hysteria" {
						out["settings"].(map[string]any)["address"] = "127.0.0.1"
					}
				}
				featureCore(t, cfg)
				b, err := newEgressBridge()
				if err != nil {
					t.Fatal(err)
				}
				clientExit := EgressSpec{Address: "127.0.0.1", Port: entry.Port, UUID: entry.Users[0].UUID, SNI: entry.SNI, PublicKey: pub, ShortID: entry.ShortID, Transport: "raw", Flow: "xtls-rprx-vision"}
				featureCore(t, map[string]any{"inbounds": []any{featureSocks(b)}, "outbounds": []any{clientExit.outbound("entry")}})
				before := hits.Load()
				beforeUDP := udpHits.Load()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				conn, err := b.dial(ctx, "tcp", direct.Listener.Addr().String())
				body := ""
				if err == nil {
					defer conn.Close()
					conn.SetDeadline(time.Now().Add(3 * time.Second))
					io.WriteString(conn, "GET / HTTP/1.1\r\nHost: gateway-test.example.com\r\nConnection: close\r\n\r\n")
					resp, e := http.ReadResponse(bufio.NewReader(conn), nil)
					if e == nil {
						bytes, e := io.ReadAll(resp.Body)
						resp.Body.Close()
						if e == nil {
							body = string(bytes)
						}
					}
				}
				if tc.valid {
					if body != "through-hy2-exit" || hits.Load() != before+1 {
						t.Fatalf("gateway did not reach HY2 exit: body=%q err=%v", body, err)
					}
				} else if body != "" || hits.Load() != before {
					t.Fatalf("failed HY2 credentials/certificate bypassed gateway: body=%q hits=%d", body, hits.Load()-before)
				}
				if bypasses.Load() != 0 {
					t.Fatal("gateway fell back to direct routing")
				}
				udpCtx, udpCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer udpCancel()
				udpTarget := direct.Listener.Addr().String()
				if len(entry.DomainRoutes) > 0 {
					_, port, _ := net.SplitHostPort(udpTarget)
					udpTarget = net.JoinHostPort("gateway-test.example.com", port)
				}
				packet, err := b.dial(udpCtx, "udp", udpTarget)
				payload := ""
				if err == nil {
					defer packet.Close()
					packet.SetDeadline(time.Now().Add(time.Second))
					if len(entry.DomainRoutes) > 0 {
						// Bridges resolve production destinations first. This fixture
						// acts as a client carrying the original SOCKS domain instead.
						udp := packet.(*egressUDPConn)
						host, port, _ := net.SplitHostPort(udpTarget)
						p, _ := strconv.Atoi(port)
						udp.header = append([]byte{0, 0, 0, 3, byte(len(host))}, []byte(host)...)
						udp.header = append(udp.header, byte(p>>8), byte(p))
					}
					packet.Write([]byte("gateway-udp"))
					buf := make([]byte, 1024)
					if len(entry.DomainRoutes) > 0 {
						n, err := packet.(*egressUDPConn).UDPConn.Read(buf)
						if err == nil && n >= 4 {
							r := bytes.NewReader(buf[4:n])
							if _, _, err := readWebsiteAddress(r, buf[3]); err == nil {
								payload = string(buf[n-r.Len() : n])
							}
						}
					} else {
						n, _ := packet.Read(buf)
						payload = string(buf[:n])
					}
				}
				if tc.valid {
					if payload != "through-hy2:gateway-udp" || udpHits.Load() != beforeUDP+1 {
						t.Fatalf("UDP did not reach selected HY2 exit: payload=%q err=%v", payload, err)
					}
					if metered {
						records := collector.Drain()
						if len(records) == 0 {
							t.Fatal("HY2 exit bypassed website accounting")
						}
						for _, r := range records {
							if r.InboundTag != entry.Tag || r.Up <= 0 || r.Down <= 0 {
								t.Fatalf("incorrect website attribution/counters: %+v", r)
							}
						}
					}
					stats, err := exec.Command(os.Getenv("XRAY_FEATURE_BINARY"), "api", "statsquery", "--server=127.0.0.1:"+strconv.Itoa(spec.APIPort)).CombinedOutput()
					if err != nil {
						t.Fatalf("stable ingress user not metered: %s %v", stats, err)
					}
					var counters struct {
						Stat []struct {
							Name  string `json:"name"`
							Value int64  `json:"value"`
						} `json:"stat"`
					}
					if err := json.Unmarshal(stats, &counters); err != nil {
						t.Fatalf("invalid user counters: %s %v", stats, err)
					}
					var userTraffic Counter
					for _, c := range counters.Stat {
						switch c.Name {
						case "user>>>stable-user>>>traffic>>>uplink":
							userTraffic.Up += c.Value
						case "user>>>stable-user>>>traffic>>>downlink":
							userTraffic.Down += c.Value
						}
					}
					if userTraffic.Up <= 0 || userTraffic.Down <= 0 {
						t.Fatalf("stable ingress user has no traffic: %+v", userTraffic)
					}
				} else if payload != "" || udpHits.Load() != beforeUDP {
					t.Fatalf("invalid TLS/auth forwarded UDP: payload=%q hits=%d", payload, udpHits.Load()-beforeUDP)
				}
			})
		}
	}
}
