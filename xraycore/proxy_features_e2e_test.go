package xraycore

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func featurePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
func featureCore(t *testing.T, cfg any) {
	t.Helper()
	binary := os.Getenv("XRAY_FEATURE_BINARY")
	if binary == "" {
		t.Skip("set XRAY_FEATURE_BINARY for isolated real-core verification")
	}
	if m, ok := cfg.(map[string]any); ok {
		m["log"] = map[string]any{"loglevel": "debug"}
	}
	raw, e := json.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, raw, 0600)
	out, e := exec.Command(binary, "run", "-test", "-c", path).CombinedOutput()
	if e != nil {
		t.Fatalf("core rejected config: %s %v", out, e)
	}
	log, e := os.Create(filepath.Join(dir, "core.log"))
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(binary, "run", "-c", path)
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		log.Close()
		if t.Failed() {
			b, _ := os.ReadFile(filepath.Join(dir, "core.log"))
			t.Log(string(b))
		}
	})
	time.Sleep(350 * time.Millisecond)
}
func featureEcho(t *testing.T, b *egressBridge) {
	t.Helper()
	tcp, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer tcp.Close()
	go func() {
		c, e := tcp.Accept()
		if e == nil {
			defer c.Close()
			io.Copy(c, c)
		}
	}()
	udp, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer udp.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, a, e := udp.ReadFromUDP(buf)
			if e != nil {
				return
			}
			udp.WriteToUDP(buf[:n], a)
		}
	}()
	for _, network := range []string{"tcp", "udp"} {
		t.Run(network, func(t *testing.T) {
			address := tcp.Addr().String()
			if network == "udp" {
				address = udp.LocalAddr().String()
			}
			c, e := b.dial(context.Background(), network, address)
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(8 * time.Second))
			payload := []byte("proxy-feature-roundtrip")
			if _, e = c.Write(payload); e != nil {
				t.Fatal(e)
			}
			buf := make([]byte, len(payload))
			if _, e = io.ReadFull(c, buf); e != nil {
				t.Fatal(e)
			}
			if string(buf) != string(payload) {
				t.Fatalf("unexpected payload %q", buf)
			}
		})
	}
}
func featureSocks(b *egressBridge) map[string]any {
	return map[string]any{"listen": "127.0.0.1", "port": b.port, "protocol": "socks", "settings": map[string]any{"auth": "password", "accounts": []any{map[string]any{"user": b.user, "pass": b.password}}, "udp": true, "ip": "127.0.0.1"}}
}
func TestFeatureBridgeRealCore(t *testing.T) {
	if os.Getenv("XRAY_FEATURE_BINARY") == "" {
		t.Skip("requires real core")
	}
	b, e := newEgressBridge()
	if e != nil {
		t.Fatal(e)
	}
	featureCore(t, map[string]any{"inbounds": []any{featureSocks(b)}, "outbounds": []any{map[string]any{"protocol": "freedom"}}})
	featureEcho(t, b)
	bad := *b
	bad.password = "wrong"
	if c, e := bad.dial(context.Background(), "tcp", "127.0.0.1:1"); e == nil {
		c.Close()
		t.Fatal("accepted invalid bridge credential")
	}
}
func TestFeatureHysteriaRealCore(t *testing.T) {
	if os.Getenv("XRAY_FEATURE_BINARY") == "" {
		t.Skip("requires real core")
	}
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	if out, e := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", key, "-out", cert, "-days", "1", "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost").CombinedOutput(); e != nil {
		t.Fatalf("certificate: %s %v", out, e)
	}
	port := featurePort(t)
	in := InboundSpec{Tag: "hy", Template: TemplateHysteria2, Port: port, SNI: "localhost", TLSCertFile: cert, TLSKeyFile: key, Users: []UserSpec{{Email: "u1", UUID: "feature-test-secret"}}}
	apiPort := featurePort(t)
	spec := Spec{Enabled: true, APIPort: apiPort, Inbounds: []InboundSpec{in}, WebsiteTraffic: &WebsiteTrafficSpec{Enabled: true}}
	collector := websiteTestCollector(t, spec)
	raw, e := BuildConfigWithWebsiteCollector(spec, collector)
	if e != nil {
		t.Fatal(e)
	}
	var cfg any
	json.Unmarshal(raw, &cfg)
	featureCore(t, cfg)
	for _, pass := range []string{"feature-test-secret", "wrong"} {
		t.Run(pass, func(t *testing.T) {
			b, e := newEgressBridge()
			if e != nil {
				t.Fatal(e)
			}
			outbound := map[string]any{"protocol": "hysteria", "settings": map[string]any{"version": 2, "address": "127.0.0.1", "port": port}, "streamSettings": map[string]any{"network": "hysteria", "security": "tls", "tlsSettings": map[string]any{"serverName": "localhost", "certificates": []any{map[string]any{"certificateFile": cert, "usage": "verify"}}}, "hysteriaSettings": map[string]any{"version": 2, "auth": pass}}}
			featureCore(t, map[string]any{"inbounds": []any{featureSocks(b)}, "outbounds": []any{outbound}})
			if pass == "feature-test-secret" {
				featureEcho(t, b)
			} else {
				c, e := b.dial(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
				if e == nil {
					defer c.Close()
					c.SetDeadline(time.Now().Add(time.Second))
					c.Write([]byte("must not forward"))
					buf := make([]byte, 1)
					if n, _ := c.Read(buf); n > 0 {
						t.Fatal("invalid auth forwarded data")
					}
				}
			}
		})
	}
	stats, err := exec.Command(os.Getenv("XRAY_FEATURE_BINARY"), "api", "statsquery", "--server=127.0.0.1:"+strconv.Itoa(apiPort)).CombinedOutput()
	if err != nil || !strings.Contains(string(stats), "user>>>u1>>>traffic>>>downlink") {
		t.Fatalf("HY2 account traffic unavailable: %s %v", stats, err)
	}
	if len(collector.Drain()) == 0 {
		t.Fatal("HY2 website traffic unavailable")
	}
	in.Users = nil
	raw, e = BuildConfig(Spec{APIPort: featurePort(t), Inbounds: []InboundSpec{in}})
	if e != nil {
		t.Fatal(e)
	}
	var empty map[string]any
	json.Unmarshal(raw, &empty)
	if len(empty["inbounds"].([]any)) != 1 {
		t.Fatal("empty user list left listener open")
	}
}

func TestFeatureFixedEgressRealCore(t *testing.T) {
	if os.Getenv("XRAY_FEATURE_BINARY") == "" {
		t.Skip("requires real core")
	}
	target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("target")) }))
	target.EnableHTTP2 = true
	target.TLS = &tls.Config{MinVersion: tls.VersionTLS13}
	target.StartTLS()
	defer target.Close()
	priv, pub, e := GenerateRealityKeys()
	if e != nil {
		t.Fatal(e)
	}
	upstreamPort := featurePort(t)
	in := InboundSpec{Tag: "exit", Template: TemplateRealityVision, Port: upstreamPort, Target: target.Listener.Addr().String(), SNI: "example.com", PrivateKey: priv, ShortID: "a1b2", Users: []UserSpec{{Email: "exit-user", UUID: "11111111-1111-4111-8111-111111111111"}}}
	raw, e := BuildConfig(Spec{APIPort: featurePort(t), Inbounds: []InboundSpec{in}})
	if e != nil {
		t.Fatal(e)
	}
	var upstream any
	json.Unmarshal(raw, &upstream)
	featureCore(t, upstream)
	for _, metered := range []bool{false, true} {
		name := "direct-routing"
		if metered {
			name = "with-website-meter"
		}
		t.Run(name, func(t *testing.T) {
			b, e := newEgressBridge()
			if e != nil {
				t.Fatal(e)
			}
			in.Tag = "entry"
			in.Port = b.port
			in.Egress = &EgressSpec{Address: "8.8.8.8", Port: upstreamPort, UUID: in.Users[0].UUID, SNI: in.SNI, PublicKey: pub, ShortID: in.ShortID, Transport: "raw", Flow: "xtls-rprx-vision"}
			spec := Spec{APIPort: featurePort(t), Inbounds: []InboundSpec{in}}
			var collector *WebsiteCollector
			if metered {
				spec.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true}
				collector = websiteTestCollector(t, spec)
			}
			raw, e := BuildConfigWithWebsiteCollector(spec, collector)
			if e != nil {
				t.Fatal(e)
			}
			var cfg map[string]any
			json.Unmarshal(raw, &cfg)
			// Only the test fixture may use loopback as an exit. Keep the generated routing and transport intact.
			for _, v := range cfg["outbounds"].([]any) {
				m := v.(map[string]any)
				if m["tag"] == "exit-entry" {
					m["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)["address"] = "127.0.0.1"
				}
			}
			inputs := cfg["inbounds"].([]any)
			for i, v := range inputs {
				if v.(map[string]any)["tag"] == "entry" {
					socks := featureSocks(b)
					socks["tag"] = "entry"
					inputs[i] = socks
				}
			}
			featureCore(t, cfg)
			featureEcho(t, b)
			if metered {
				records := collector.Drain()
				if len(records) == 0 {
					t.Fatal("egress bypassed website accounting")
				}
			}
		})
	}
}
