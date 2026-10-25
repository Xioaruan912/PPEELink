package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"github.com/gin-gonic/gin"
	"math/big"
	"net/http/httptest"
	"ppeelink/models"
	"ppeelink/xraycore"
	"testing"
	"time"
)

func autoPublic(t *testing.T) string {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), DNSNames: []string{xraycore.AutoTLSServerName}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, e := x509.CreateCertificate(rand.Reader, c, c, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
func TestCertificateReportBinding(t *testing.T) {
	s, in := setupProxyDB(t)
	in.Template = xraycore.TemplateHysteria2
	in.TLSMode = "auto"
	in.TLSAutoID = "0123456789abcdef0123456789abcdef"
	in.SNI = xraycore.AutoTLSServerName
	models.DB.Save(&in)
	if len(serverNodes(s, models.ProxyAccount{UUID: "secret"})) != 0 {
		t.Fatal("pending node published")
	}
	public := autoPublic(t)
	report := xraycore.Report{Certificates: map[string]string{in.TLSAutoID: public}}
	if changed, e := applyCertificateReports(models.DB, s.ID+1, report); e != nil || changed {
		t.Fatal("other server changed certificate")
	}
	if changed, e := applyCertificateReports(models.DB, s.ID, report); e != nil || !changed {
		t.Fatalf("first report rejected: %v", e)
	}
	if len(serverNodes(s, models.ProxyAccount{UUID: "secret"})) != 1 {
		t.Fatal("ready node absent")
	}
	if changed, e := applyCertificateReports(models.DB, s.ID, report); e != nil || changed {
		t.Fatal("duplicate report not idempotent")
	}
	report.Certificates[in.TLSAutoID] = autoPublic(t)
	if _, e := applyCertificateReports(models.DB, s.ID, report); e == nil {
		t.Fatal("certificate silently replaced")
	}
}

func TestAutoTLSSaveCapabilityAndIdentity(t *testing.T) {
	server, _ := setupProxyDB(t)
	server.Kind = models.ServerKindAgent
	server.PublicHost = "192.0.2.1"
	server.ProxyFeatures = 1
	models.DB.Save(&server)
	req := inboundSaveRequest{ServerID: server.ID, Template: xraycore.TemplateHysteria2, TLSMode: "auto", Port: 24444, Enabled: true}
	save := func() int {
		raw, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		ProxyInboundSave(c)
		return w.Code
	}
	if save() != 400 {
		t.Fatal("old Agent accepted automatic TLS")
	}
	server.ProxyFeatures = 2
	models.DB.Save(&server)
	if save() != 200 {
		t.Fatal("automatic TLS rejected")
	}
	var in models.ProxyInbound
	models.DB.Where("server_id = ? AND port = ?", server.ID, req.Port).First(&in)
	if len(in.TLSAutoID) != 32 || in.SNI != xraycore.AutoTLSServerName || in.TLSCertFile != "" {
		t.Fatal("invalid automatic identity")
	}
	identity := in.TLSAutoID
	req.ID = in.ID
	req.Remark = "renamed"
	if save() != 200 {
		t.Fatal("edit rejected")
	}
	models.DB.First(&in, in.ID)
	if in.TLSAutoID != identity {
		t.Fatal("ordinary edit rotated identity")
	}
	server.PublicHost = "example.com"
	models.DB.Save(&server)
	if save() != 400 {
		t.Fatal("domain address accepted in domainless mode")
	}
}
