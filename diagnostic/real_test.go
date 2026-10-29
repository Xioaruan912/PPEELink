package diagnostic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"ppeelink/certpin"
	"ppeelink/node"
	"ppeelink/xraycore"
	"strings"
	"testing"
	"time"
)

func free(t *testing.T) int {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
func TestRealDiagnostics(t *testing.T) {
	binary := os.Getenv("XRAY_FEATURE_BINARY")
	if binary == "" {
		t.Skip("requires Xray")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer target.Close()
	old := plainProbeURL
	plainProbeURL = target.URL
	defer func() { plainProbeURL = old }()
	fake := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	fake.EnableHTTP2 = true
	fake.TLS = &tls.Config{MinVersion: tls.VersionTLS13}
	fake.StartTLS()
	defer fake.Close()
	for _, kind := range []string{"raw", "xhttp", "hy2"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			port := free(t)
			private, pub, e := xraycore.GenerateRealityKeys()
			if e != nil {
				t.Fatal(e)
			}
			uuid := "11111111-1111-4111-8111-111111111111"
			in := xraycore.InboundSpec{Tag: "test", Port: port, Template: xraycore.TemplateRealityVision, Target: fake.Listener.Addr().String(), SNI: "www.example.com", PrivateKey: private, ShortID: "abcd", Users: []xraycore.UserSpec{{Email: "test", UUID: uuid}}}
			flow := "xtls-rprx-vision"
			if kind == "xhttp" {
				in.Template = xraycore.TemplateXHTTPReality
				in.Path = "/diagnose"
				flow = ""
			}
			relayIn := in
			relayIn.Tag = "relay"
			relayIn.Port = free(t)
			relayIn.Template = xraycore.TemplateRealityVision
			relayLink := node.EncodeVLESSURL(node.VLESS{Uuid: uuid, Server: "127.0.0.1", Port: relayIn.Port, Query: node.VLESSQuery{Security: "reality", Type: "raw", Sni: in.SNI, Pbk: pub, Sid: in.ShortID, Flow: "xtls-rprx-vision"}})
			var link string
			if kind == "hy2" {
				key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				c := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"hy2.subl.invalid"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
				der, e := x509.CreateCertificate(rand.Reader, c, c, &key.PublicKey, key)
				if e != nil {
					t.Fatal(e)
				}
				public := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
				kd, _ := x509.MarshalPKCS8PrivateKey(key)
				in.Template = xraycore.TemplateHysteria2
				in.SNI = "hy2.subl.invalid"
				in.TLSCertFile = filepath.Join(dir, "cert.pem")
				in.TLSKeyFile = filepath.Join(dir, "key.pem")
				os.WriteFile(in.TLSCertFile, []byte(public), 0600)
				os.WriteFile(in.TLSKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kd}), 0600)
				_, pin, _ := certpin.Parse(public)
				link = node.EncodeHY2URL(node.HY2{Host: "127.0.0.1", Port: port, Password: uuid, Sni: in.SNI, PinSHA256: pin, Insecure: 1, Certificate: public})
			} else {
				link = node.EncodeVLESSURL(node.VLESS{Uuid: uuid, Server: "127.0.0.1", Port: port, Query: node.VLESSQuery{Security: "reality", Type: kind, Sni: in.SNI, Pbk: pub, Sid: in.ShortID, Flow: flow, Path: in.Path}})
			}
			cfg, e := xraycore.BuildConfig(xraycore.Spec{Enabled: true, APIPort: free(t), Inbounds: []xraycore.InboundSpec{in, relayIn}})
			if e != nil {
				t.Fatal(e)
			}
			f := filepath.Join(dir, "server.json")
			os.WriteFile(f, cfg, 0600)
			cmd := exec.Command(binary, "run", "-c", f)
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() { cmd.Process.Kill(); cmd.Wait() }()
			time.Sleep(200 * time.Millisecond)
			for _, bad := range []bool{false, true} {
				testLink := link
				if bad {
					testLink = strings.ReplaceAll(link, uuid, "22222222-2222-4222-8222-222222222222")
				}
				p, e := parse(testLink)
				if e != nil {
					t.Fatal(e)
				}
				r := &Result{Steps: []Step{}}
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				_, e = runCore(ctx, r, p, binary, target.URL, func(k, title, status, detail string, start time.Time) {
					r.Steps = append(r.Steps, Step{k, title, status, detail, time.Since(start).Milliseconds()})
				})
				cancel()
				if e != nil {
					t.Fatal(e)
				}
				last := r.Steps[len(r.Steps)-1]
				if !bad && last.Status != "passed" {
					t.Fatalf("valid %s failed: %+v", kind, r)
				}
				if bad && last.Status == "passed" {
					t.Fatal("invalid credential passed")
				}
				raw, _ := json.Marshal(r)
				if strings.Contains(string(raw), uuid) || strings.Contains(string(raw), private) {
					t.Fatal("credentials leaked")
				}
			}
			for _, badRelay := range []bool{false, true} {
				testLink := relayLink
				if badRelay {
					testLink = strings.ReplaceAll(relayLink, uuid, "22222222-2222-4222-8222-222222222222")
				}
				relay, e := parse(testLink)
				if e != nil {
					t.Fatal(e)
				}
				p, e := parse(link)
				p.relays = []profile{relay}
				if e != nil {
					t.Fatal(e)
				}
				r := &Result{Steps: []Step{}}
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				_, e = runCore(ctx, r, p, binary, target.URL, func(k, title, status, detail string, start time.Time) {
					r.Steps = append(r.Steps, Step{k, title, status, detail, time.Since(start).Milliseconds()})
				})
				cancel()
				if e != nil {
					t.Fatal(e)
				}
				last := r.Steps[len(r.Steps)-1]
				if !badRelay && last.Status != "passed" {
					t.Fatalf("valid chain %s failed: %+v", kind, r)
				}
				if badRelay && last.Status == "passed" {
					t.Fatal("broken relay was bypassed")
				}
				raw, _ := json.Marshal(r)
				if strings.Contains(string(raw), uuid) || strings.Contains(string(raw), private) {
					t.Fatal("credentials leaked")
				}
			}
		})
	}
}
