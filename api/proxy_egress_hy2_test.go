package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"net/url"
	"ppeelink/certpin"
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/xraycore"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNodeEgressAllowsSecureHY2PhysicalNode(t *testing.T) {
	flags := []byte(`{"dialerProxyId":42}`)
	n := models.Node{Link: "hy2://user:pass@exit.example.com:24443?sni=exit.example.com&alpn=h3&obfs=salamander&obfs-password=secret&pinSHA256=" + strings.Repeat("ab", 32) + "&insecure=1", Flags: string(flags)}
	before := n.Flags
	e, err := nodeEgress(n)
	if err != nil {
		t.Fatalf("valid physical HY2 node rejected: %v", err)
	}
	raw, _ := json.Marshal(e)
	var cfg map[string]any
	json.Unmarshal(raw, &cfg)
	if cfg["protocol"] != "hysteria2" || cfg["password"] != "user:pass" || cfg["obfsPassword"] != "secret" {
		t.Fatalf("node parameters lost: %s", raw)
	}
	if n.Flags != before {
		t.Fatal("client-side dialer settings mutated")
	}
}
func TestNodeEgressHY2DefaultSNIAndEscapedPassword(t *testing.T) {
	e, err := nodeEgress(models.Node{Link: "hysteria2://a%40b%3Ac@exit.example.com:443"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(e)
	var cfg map[string]any
	json.Unmarshal(raw, &cfg)
	if cfg["sni"] != "exit.example.com" || cfg["password"] != "a@b:c" {
		t.Fatal(string(raw))
	}
}
func TestNodeEgressHY2RejectsUnsafeOrUnsupportedLinkSettings(t *testing.T) {
	base := "hy2://secret@exit.example.com:443?"
	for _, q := range []string{"insecure=1", "insecure=true", "insecure=garbage", "pinSHA256=bad", "subl-cert=bad", "pinSHA256=" + strings.Repeat("ab", 32) + "&subl-cert=" + url.QueryEscape("bm90IGEgcGVt"), "obfs=unknown", "obfs=salamander", "obfs-password=orphan", "alpn=h3,", "upmbps=3", "downmbps=3", "peer=other.example.com", "auth=other", "mport=443,444", "extra=1", "sni=one.example.com&sni=two.example.com", "unknown=ignored"} {
		t.Run(q, func(t *testing.T) {
			if _, err := nodeEgress(models.Node{Link: base + q}); err == nil {
				t.Fatal("unsafe/unsupported link silently accepted")
			}
		})
	}
	for _, flags := range []node.NodeFlags{{SkipCertVerify: true}, {TFO: true}} {
		raw, _ := json.Marshal(flags)
		if _, err := nodeEgress(models.Node{Link: base, Flags: string(raw)}); err == nil {
			t.Fatal("unsupported flag accepted")
		}
	}
}
func TestNodeEgressRealityAllowsClientDialerWithoutChaining(t *testing.T) {
	raw := []byte(`{"dialerProxyId":999}`)
	e, err := nodeEgress(models.Node{Flags: string(raw), Link: "vless://11111111-1111-4111-8111-111111111111@exit.example.com:24443?security=reality&type=tcp&sni=www.example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	if err != nil || e == nil {
		t.Fatalf("physical Reality exit with client dialer rejected: %v", err)
	}
}

func TestNodeEgressRealityRejectsDroppedParameters(t *testing.T) {
	base := "vless://11111111-1111-4111-8111-111111111111@exit.example.com:24443?security=reality&type=tcp&sni=www.example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	for _, extra := range []string{"&extra=%7B%7D", "&pqv=required", "&spx=/custom", "&allowInsecure=1", "&security=tls", "&sni=other.example.com", "&broken=%zz", "&broken=a;b", "&path=/unused"} {
		t.Run(extra, func(t *testing.T) {
			if _, err := nodeEgress(models.Node{Link: base + extra}); err == nil {
				t.Fatal("unsupported or ambiguous Reality parameters silently dropped")
			}
		})
	}
	for _, link := range []string{strings.Replace(base, "@exit", ":ignored@exit", 1), strings.Replace(base, ":24443?", ":24443/ignored?", 1)} {
		if _, err := nodeEgress(models.Node{Link: link}); err == nil {
			t.Fatal("unsupported Reality URL component silently dropped")
		}
	}
}

func TestNodeEgressRejectsPureRelayGeneratedNodes(t *testing.T) {
	for _, source := range []string{"agent-relay:1", "agent-relay:99"} {
		if _, err := nodeEgress(models.Node{Source: source, Link: "hy2://secret@exit.example.com:443"}); err == nil {
			t.Fatal("pure relay node accepted as gateway exit")
		}
	}
}

// This catches accepting an embedded certificate with a different pin, SNI, or validity period.
func TestNodeEgressHY2ValidatesEmbeddedCertificate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"exit.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	public := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	_, pin, err := certpin.Parse(public)
	if err != nil {
		t.Fatal(err)
	}
	link := node.EncodeHY2URL(node.HY2{Host: "exit.example.com", Port: 443, Password: "secret", Sni: "exit.example.com", PinSHA256: pin, Certificate: public, Insecure: 1})
	if _, err := nodeEgress(models.Node{Link: link}); err != nil {
		t.Fatalf("matching certificate rejected: %v", err)
	}
	u, _ := url.Parse(link)
	q := u.Query()
	q.Del("sni")
	u.RawQuery = q.Encode()
	if e, err := nodeEgress(models.Node{Link: u.String()}); err != nil || e.SNI != "exit.example.com" {
		t.Fatalf("embedded certificate with implicit SNI rejected: %v", err)
	}
	for _, change := range []struct{ key, value string }{{"pinSHA256", strings.Repeat("ab", 32)}, {"sni", "other.example.com"}, {"subl-cert", base64.RawURLEncoding.EncodeToString([]byte("invalid PEM"))}} {
		u, _ := url.Parse(link)
		q := u.Query()
		q.Set(change.key, change.value)
		u.RawQuery = q.Encode()
		if _, err := nodeEgress(models.Node{Link: u.String()}); err == nil {
			t.Fatalf("invalid %s accepted", change.key)
		}
	}
	for _, future := range []bool{false, true} {
		template.NotBefore = time.Now().Add(-2 * time.Hour)
		template.NotAfter = time.Now().Add(-time.Hour)
		if future {
			template.NotBefore, template.NotAfter = time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)
		}
		der, err = x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(link)
		q := u.Query()
		// Keep the pin correct so this exercises validity rather than pin mismatch.
		sum := sha256.Sum256(der)
		q.Set("pinSHA256", hex.EncodeToString(sum[:]))
		q.Set("subl-cert", base64.RawURLEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
		u.RawQuery = q.Encode()
		if _, err := nodeEgress(models.Node{Link: u.String()}); err == nil {
			t.Fatalf("certificate outside validity period accepted (future=%t)", future)
		}
	}
}

func TestHY2GatewayEditPreservesIngressCredentialsAndQuota(t *testing.T) {
	for _, template := range []string{xraycore.TemplateRealityVision, xraycore.TemplateXHTTPReality, xraycore.TemplateHysteria2} {
		t.Run(template, func(t *testing.T) {
			server, in := setupProxyDB(t)
			server.PublicHost, server.ProxyFeatures = "8.8.4.4", 5
			if err := models.DB.Save(&server).Error; err != nil {
				t.Fatal(err)
			}
			in.Template = template
			if template == xraycore.TemplateXHTTPReality {
				in.Path = "/stable-entry"
			}
			if template == xraycore.TemplateHysteria2 {
				in.TLSMode, in.TLSAutoID = "auto", "0123456789abcdef0123456789abcdef"
				in.SNI, in.TLSCertificate = xraycore.AutoTLSServerName, autoPublic(t)
				_, in.TLSFingerprint, _ = certpin.Parse(in.TLSCertificate)
			}
			previousExit := models.Node{Name: "Reality physical exit", Link: "vless://11111111-1111-4111-8111-111111111111@9.9.9.9:24444?security=reality&type=tcp&sni=www.example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
			if err := models.DB.Create(&previousExit).Error; err != nil {
				t.Fatal(err)
			}
			in.EgressNodeID = previousExit.ID
			if err := models.DB.Save(&in).Error; err != nil {
				t.Fatal(err)
			}
			active := addAccount(t, "gateway-active", nil)
			addAccount(t, "gateway-quota-exhausted", func(a *models.ProxyAccount) { a.TrafficLimit, a.TrafficUsed = 100, 100 })
			beforeLink := InboundLink(server, in, active, "stable-entry")
			beforeSpec, err := BuildServerSpec(server)
			if err != nil {
				t.Fatal(err)
			}
			physical := models.Node{Name: "HY2 physical exit", Link: "hy2://exit-secret@1.1.1.1:24444?sni=exit.example.com", Flags: `{"dialerProxyId":999}`}
			if err := models.DB.Create(&physical).Error; err != nil {
				t.Fatal(err)
			}
			req := inboundSaveRequest{ID: in.ID, ServerID: server.ID, Remark: in.Remark, Template: in.Template, TLSMode: in.TLSMode, Port: in.Port, Target: in.Target, SNI: in.SNI, Path: in.Path, Enabled: true}
			save := func() *httptest.ResponseRecorder {
				raw, err := json.Marshal(req)
				if err != nil {
					t.Fatal(err)
				}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("POST", "/", bytes.NewReader(raw))
				c.Request.Header.Set("Content-Type", "application/json")
				ProxyInboundSave(c)
				return w
			}
			// Both default and website-route exits require feature 5. Rejected
			// edits must not change the existing entry or its TLS identity.
			server.ProxyFeatures = 4
			models.DB.Save(&server)
			for _, route := range []bool{false, true} {
				req.EgressNodeID, req.DomainRoutes = physical.ID, nil
				if route {
					req.EgressNodeID = 0
					req.DomainRoutes = []models.DomainRoute{{Domain: "example.com", Match: "suffix", EgressNodeID: physical.ID}}
				}
				if w := save(); w.Code != 400 {
					t.Fatalf("old Agent accepted HY2 gateway: %d %s", w.Code, w.Body.String())
				}
			}
			var stored models.ProxyInbound
			if err := models.DB.First(&stored, in.ID).Error; err != nil || !reflect.DeepEqual(stored, in) {
				t.Fatalf("rejected edit changed stable entry: %v", err)
			}
			server.ProxyFeatures = 5
			models.DB.Save(&server)
			req.EgressNodeID, req.DomainRoutes = physical.ID, nil
			if w := save(); w.Code != 200 {
				t.Fatalf("HY2 gateway save: %d %s", w.Code, w.Body.String())
			}
			if err := models.DB.First(&stored, in.ID).Error; err != nil {
				t.Fatal(err)
			}
			if after := InboundLink(server, stored, active, "stable-entry"); after != beforeLink {
				t.Fatalf("gateway edit rotated subscription credentials:\nbefore %s\nafter %s", beforeLink, after)
			}
			stored.EgressNodeID = in.EgressNodeID
			if !reflect.DeepEqual(stored, in) {
				t.Fatal("changing physical exit altered the stable ingress configuration")
			}
			afterSpec, err := BuildServerSpec(server)
			if err != nil || len(afterSpec.Inbounds) != 1 || afterSpec.Inbounds[0].Egress == nil || !afterSpec.Inbounds[0].Egress.IsHY2() {
				t.Fatalf("HY2 exit missing from dispatched spec: %+v %v", afterSpec.Inbounds, err)
			}
			if !reflect.DeepEqual(afterSpec.Inbounds[0].Users, beforeSpec.Inbounds[0].Users) || len(afterSpec.Inbounds[0].Users) != 1 || afterSpec.Inbounds[0].Users[0].UUID != active.UUID {
				t.Fatal("gateway edit changed active users or admitted an exhausted account")
			}
		})
	}
}
