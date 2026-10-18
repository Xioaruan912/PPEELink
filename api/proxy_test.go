package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/xraycore"
)

func setupProxyDB(t *testing.T) (models.ProxyServer, models.ProxyInbound) {
	t.Helper()
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProxyInbound{}, &models.ProxyTraffic{}, &models.ProxyAccount{}); err != nil {
		t.Fatal(err)
	}
	priv, pub, _ := xraycore.GenerateRealityKeys()
	server := models.ProxyServer{Name: "JP", Kind: models.ServerKindAgent, PublicHost: "jp.example.com", Enabled: true,
		AgentToken: models.HashAgentToken("agent-secret")}
	models.DB.Create(&server)
	in := models.ProxyInbound{ServerID: server.ID, Remark: "Reality", Template: xraycore.TemplateRealityVision, Port: 24443,
		Target: "www.apple.com:443", SNI: "www.apple.com", PrivateKey: priv, PublicKey: pub, ShortID: "abcd1234", Enabled: true}
	models.DB.Create(&in)
	return server, in
}

func addAccount(t *testing.T, username string, mutate func(*models.ProxyAccount)) models.ProxyAccount {
	t.Helper()
	u := &models.User{Username: username, Password: "long-password", Role: models.RoleUser}
	if err := u.Create(); err != nil {
		t.Fatal(err)
	}
	acc, err := models.EnsureProxyAccount(models.DB, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	acc.Enabled = true
	if mutate != nil {
		mutate(&acc)
	}
	models.DB.Save(&acc)
	return acc
}

func TestBuildServerSpecRespectsQuota(t *testing.T) {
	server, _ := setupProxyDB(t)
	ok := addAccount(t, "alice", nil)
	addAccount(t, "bob", func(a *models.ProxyAccount) { a.TrafficLimit, a.TrafficUsed = 100, 100 })
	past := time.Now().Add(-time.Hour)
	addAccount(t, "carol", func(a *models.ProxyAccount) { a.ExpireAt = &past })
	spec, err := BuildServerSpec(server)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Inbounds) != 1 || len(spec.Inbounds[0].Users) != 1 || spec.Inbounds[0].Users[0].UUID != ok.UUID {
		t.Fatalf("only the account within quota and validity must be included: %+v", spec.Inbounds)
	}
	if _, err := xraycore.BuildConfig(spec); err != nil {
		t.Fatalf("generated config invalid: %v", err)
	}
	if !spec.BlockPrivate || !spec.BlockBitTorrent {
		t.Fatal("abuse protections must be on")
	}
}

func TestApplyReportTrafficAndDeviceLimit(t *testing.T) {
	server, _ := setupProxyDB(t)
	acc := addAccount(t, "dave", func(a *models.ProxyAccount) { a.DeviceLimit = 2 })
	report := xraycore.Report{Running: true, Traffic: map[string]xraycore.Counter{acc.Email(): {Up: 1000, Down: 9000}},
		Online: map[string]int{acc.Email(): 3}}
	if err := ApplyServerReport(server, report); err != nil {
		t.Fatal(err)
	}
	if err := ApplyServerReport(server, xraycore.Report{Traffic: map[string]xraycore.Counter{acc.Email(): {Up: 1, Down: 1}}}); err != nil {
		t.Fatal(err)
	}
	var got models.ProxyAccount
	models.DB.First(&got, "user_id = ?", acc.UserID)
	if got.TrafficUsed != 10002 {
		t.Fatalf("traffic not accumulated: %d", got.TrafficUsed)
	}
	if got.BlockedUntil == nil || got.Active(time.Now()) {
		t.Fatal("exceeding the device limit must block the account temporarily")
	}
	var day models.ProxyTraffic
	models.DB.First(&day, "user_id = ?", acc.UserID)
	if day.Up != 1001 || day.Down != 9001 {
		t.Fatalf("daily record wrong: %+v", day)
	}
}

func TestMonthlyReset(t *testing.T) {
	setupProxyDB(t)
	old := time.Now().AddDate(0, -2, 0)
	acc := addAccount(t, "erin", func(a *models.ProxyAccount) { a.ResetDay, a.TrafficUsed, a.LastResetAt = 1, 500, &old })
	resetDueAccounts(models.DB, time.Now())
	var got models.ProxyAccount
	models.DB.First(&got, "user_id = ?", acc.UserID)
	if got.TrafficUsed != 0 || got.LastResetAt == nil {
		t.Fatalf("usage must reset on the reset day: %+v", got)
	}
}

func TestInboundLinkParsesBack(t *testing.T) {
	server, in := setupProxyDB(t)
	acc := addAccount(t, "frank", nil)
	link := InboundLink(server, in, acc, "JP Reality")
	p, err := node.ParseOutbound(link)
	if err != nil || p.Server != "jp.example.com" || p.Port != 24443 {
		t.Fatalf("link not parseable: %s %v %+v", link, err, p)
	}
	for _, want := range []string{"security=reality", "pbk=" + in.PublicKey, "flow=xtls-rprx-vision", "sid=abcd1234", acc.UUID} {
		if !strings.Contains(link, want) {
			t.Fatalf("link missing %q: %s", want, link)
		}
	}
	proxies := node.BuildClashProxies([]string{link}, nil, node.SqlConfig{})
	if len(proxies) != 1 || proxies[0].Reality_opts["public-key"] != in.PublicKey {
		t.Fatalf("clash conversion lost reality options: %+v", proxies)
	}
}

func TestAgentAuthAndSpec(t *testing.T) {
	setupProxyDB(t)
	addAccount(t, "gina", nil)
	call := func(token string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent/spec", nil)
		if token != "" {
			c.Request.Header.Set("Authorization", "Agent "+token)
		}
		AgentSpec(c)
		return rec
	}
	if rec := call("wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token must be rejected, got %d", rec.Code)
	}
	rec := call("agent-secret")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"privateKey"`) {
		t.Fatalf("agent spec missing: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPersonalSubscription(t *testing.T) {
	setupProxyDB(t)
	acc := addAccount(t, "hank", func(a *models.ProxyAccount) { a.TrafficLimit, a.TrafficUsed = 1<<30, 1<<20 })
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/p/?token="+acc.SubToken+"&client=v2ray", nil)
	GetPersonalSubscription(c)
	if rec.Code != 200 {
		t.Fatalf("personal subscription failed: %d %s", rec.Code, rec.Body.String())
	}
	decoded, _ := base64.StdEncoding.DecodeString(rec.Body.String())
	if !strings.Contains(string(decoded), acc.UUID) {
		t.Fatalf("subscription must carry the account UUID: %s", decoded)
	}
	if info := rec.Header().Get("Subscription-Userinfo"); !strings.Contains(info, "download=1048576; total=1073741824") {
		t.Fatalf("userinfo wrong: %q", info)
	}
}
