package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestShouldApplyAppRulesAutoSkipsIOSAndUnsupportedClients(t *testing.T) {
	if shouldApplyAppRules("loon", "Loon/3.2.1") {
		t.Fatal("loon must skip app rules")
	}
	if shouldApplyAppRules("shadowrocket", "Shadowrocket/2100 CFNetwork/1498") {
		t.Fatal("shadowrocket must skip app rules")
	}
	if shouldApplyAppRules("surge", "Surge iOS/3200") {
		t.Fatal("surge must skip app rules")
	}
	if shouldApplyAppRules("stash", "Stash/2.6.0") {
		t.Fatal("stash must skip app rules")
	}
	if shouldApplyAppRules("clash", "ClashMi/1.0 (iPhone; iOS 18.0)") {
		t.Fatal("iOS clash UA must skip app rules")
	}
	if shouldApplyAppRules("singbox", "SFI/1.11.0 (iOS)") {
		t.Fatal("iOS sing-box (SFI) must skip app rules")
	}
	if !shouldApplyAppRules("clash", "clash-verge/v2.2.0") {
		t.Fatal("desktop clash-verge must apply app rules")
	}
	if !shouldApplyAppRules("singbox", "sing-box 1.11.4") {
		t.Fatal("desktop/android sing-box must apply app rules")
	}
}

func TestServeSubscriptionWithAppRulesSucceedsForLoonShadowrocketAndIOSClash(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Subcription{}, &models.SubLogs{}, &models.Node{}, &models.GroupNode{},
		&models.NodeHealthEvent{}, &models.AlertSetting{},
		&models.SubscriptionArtifact{}, &models.SubscriptionArtifactPointer{},
		&models.ProxyServer{},
	); err != nil {
		t.Fatal(err)
	}
	oldDB := models.DB
	models.DB = db
	defer func() { models.DB = oldDB }()

	tmp := t.TempDir()
	clashTpl := filepath.Join(tmp, "clash.yaml")
	loonTpl := filepath.Join(tmp, "loon.conf")
	_ = os.WriteFile(clashTpl, []byte("proxies: ~\nproxy-groups:\n  - name: 日常使用\n    type: select\n    proxies: []\nrules:\n  - MATCH,日常使用\n"), 0644)
	_ = os.WriteFile(loonTpl, []byte("[General]\n[Proxy]\n[Proxy Group]\nProxy = select, DIRECT\n[Rule]\nFINAL,Proxy\n"), 0644)

	n := models.Node{
		Name: "JP-01",
		Link: "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?encryption=none&security=tls&sni=example.com#JP-01",
	}
	db.Create(&n)
	sub := models.Subcription{
		Name:  "test-sub",
		Token: "tok1234567890",
		Config: `{"clash":"` + clashTpl + `","loon":"` + loonTpl + `","udp":true,"appRules":[{"platform":"all","preset":"chatgpt","label":"ChatGPT / Codex","processes":["ChatGPT.exe","codex.exe"],"target":"PROXY"}]}`,
		Nodes: []models.Node{n},
	}
	if err := sub.Add(); err != nil {
		t.Fatal(err)
	}

	runReq := func(client, ua string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?token="+sub.Token+"&client="+client, nil)
		c.Request.Header.Set("User-Agent", ua)
		serveSubscriptionClient(c, sub, client)
		return w
	}

	// 1. Loon must succeed (HTTP 200) and skip app rules
	wLoon := runReq("loon", "Loon/3.2.0 CFNetwork/1498")
	if wLoon.Code != http.StatusOK {
		t.Fatalf("loon expected 200, got %d: %s", wLoon.Code, wLoon.Body.String())
	}
	if wLoon.Header().Get("X-PPEELink-AppRules") != "skipped" {
		t.Fatalf("expected X-PPEELink-AppRules=skipped for loon")
	}

	// 2. Shadowrocket must succeed (HTTP 200) and skip app rules
	wSR := runReq("shadowrocket", "Shadowrocket/2100 CFNetwork/1498")
	if wSR.Code != http.StatusOK {
		t.Fatalf("shadowrocket expected 200, got %d: %s", wSR.Code, wSR.Body.String())
	}
	if wSR.Header().Get("X-PPEELink-AppRules") != "skipped" {
		t.Fatalf("expected X-PPEELink-AppRules=skipped for shadowrocket")
	}

	// 3. Clash fetched with iOS User-Agent must succeed WITHOUT PROCESS-NAME rules
	wIOSClash := runReq("clash", "ClashMi/1.0 (iPhone; iOS 18.1)")
	if wIOSClash.Code != http.StatusOK {
		t.Fatalf("iOS clash expected 200, got %d: %s", wIOSClash.Code, wIOSClash.Body.String())
	}
	if strings.Contains(wIOSClash.Body.String(), "PROCESS-NAME,ChatGPT.exe") {
		t.Fatalf("iOS clash must not contain PROCESS-NAME rules: %s", wIOSClash.Body.String())
	}

	// 4. Desktop Clash must still include PROCESS-NAME rules
	wDeskClash := runReq("clash", "clash-verge/v2.2.0")
	if wDeskClash.Code != http.StatusOK {
		t.Fatalf("desktop clash expected 200, got %d: %s", wDeskClash.Code, wDeskClash.Body.String())
	}
	if !strings.Contains(wDeskClash.Body.String(), "PROCESS-NAME,ChatGPT.exe") || !strings.Contains(wDeskClash.Body.String(), "PROCESS-NAME,codex.exe") {
		t.Fatalf("desktop clash missing PROCESS-NAME rules: %s", wDeskClash.Body.String())
	}
}
