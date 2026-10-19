package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ppeelink/middlewares"
	"ppeelink/models"
	"ppeelink/node"
)

const (
	testLinkA = "trojan://pass@a.example.com:443?sni=a.example.com#Alpha"
	testLinkB = "trojan://pass@b.example.com:443?sni=b.example.com#Bravo"
)

// setupTestDB 使用内存库并迁移本文件测试涉及的表。
func setupTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	old := models.DB
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Subcription{}, &models.Node{}, &models.GroupNode{},
		&models.SubLogs{}, &models.SubscriptionArtifact{}, &models.SubscriptionArtifactPointer{}, &models.APIToken{}); err != nil {
		t.Fatal(err)
	}
	models.DB = db
	t.Cleanup(func() { models.DB = old })
	if len(middlewares.Secret) == 0 {
		middlewares.Secret = []byte("test-secret")
	}
}

func jsonRequest(t *testing.T, handler gin.HandlerFunc, body any, setup func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	if setup != nil {
		setup(c)
	}
	handler(c)
	return rec
}

func TestMiddlewareForcesPasswordChangeAndRoles(t *testing.T) {
	setupTestDB(t)
	admin := &models.User{Username: "admin", Password: models.DefaultPassword, Role: models.RoleAdmin, MustChangePassword: true}
	viewer := &models.User{Username: "viewer", Password: "longpassword", Role: models.RoleUser}
	for _, u := range []*models.User{admin, viewer} {
		if err := u.Create(); err != nil {
			t.Fatal(err)
		}
	}
	check := func(username, path string) int {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		if !middlewares.AuthorizeUser(c, username) {
			return rec.Code
		}
		return http.StatusOK
	}
	if got := check("admin", "/api/v1/subcription/get"); got != http.StatusForbidden {
		t.Fatalf("admin with default password must be blocked, got %d", got)
	}
	if got := check("admin", "/api/v1/users/update"); got != http.StatusOK {
		t.Fatalf("password change must stay reachable, got %d", got)
	}
	if got := check("viewer", "/api/v1/nodes/get"); got != http.StatusForbidden {
		t.Fatalf("user role must not reach admin API, got %d", got)
	}
	if got := check("viewer", "/api/v1/my/subscriptions"); got != http.StatusOK {
		t.Fatalf("user role must reach my subscriptions, got %d", got)
	}
	_ = models.DB.Model(&models.User{}).Where("username = ?", "viewer").Update("disabled", true).Error
	if got := check("viewer", "/api/v1/my/subscriptions"); got != http.StatusUnauthorized {
		t.Fatalf("disabled user must be rejected, got %d", got)
	}
}

func TestVerifyFlagsDefaultPassword(t *testing.T) {
	setupTestDB(t)
	u := &models.User{Username: "root", Password: models.DefaultPassword, Role: models.RoleAdmin}
	if err := u.Create(); err != nil {
		t.Fatal(err)
	}
	login := &models.User{Username: "root", Password: models.DefaultPassword}
	if err := login.Verify(); err != nil || !login.MustChangePassword {
		t.Fatalf("default password login must require change: %v %+v", err, login)
	}
	if err := login.Set(&models.User{Password: "a-better-password"}); err != nil {
		t.Fatal(err)
	}
	again := &models.User{Username: "root", Password: "a-better-password"}
	if err := again.Verify(); err != nil || again.MustChangePassword {
		t.Fatalf("password change must clear the flag: %v %+v", err, again)
	}
}

func TestGetClientStatusCodesAndTraffic(t *testing.T) {
	setupTestDB(t)
	expired := time.Now().Add(-time.Hour)
	updated := time.Now()
	sourceExpire := time.Now().Add(48 * time.Hour)
	active := models.Subcription{Name: "active", Token: "aaaa1111", Config: `{"udp":true}`, SourceURLs: "https://example.com/sub",
		SourceUpload: 10, SourceDownload: 20, SourceTotal: 1000, SourceExpire: &sourceExpire, SourceTrafficAt: &updated}
	old := models.Subcription{Name: "old", Token: "bbbb2222", ExpiresAt: &expired}
	models.DB.Create(&active)
	models.DB.Create(&old)

	call := func(token string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/c/?token="+token+"&client=v2ray", nil)
		GetClient(c)
		return rec
	}
	if rec := call("nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token: want 404, got %d", rec.Code)
	}
	if rec := call("BBBB2222"); rec.Code != http.StatusGone {
		t.Fatalf("expired subscription: want 410, got %d", rec.Code)
	}
	header := subscriptionUserinfo(active)
	if !strings.Contains(header, "upload=10; download=20; total=1000") || !strings.Contains(header, "expire=") {
		t.Fatalf("traffic header not aggregated: %q", header)
	}
}

func TestNodeBatchRenameAndChain(t *testing.T) {
	setupTestDB(t)
	a := models.Node{Name: "Alpha", Link: testLinkA}
	b := models.Node{Name: "Bravo", Link: testLinkB}
	models.DB.Create(&a)
	models.DB.Create(&b)
	models.DB.Create(&models.Subcription{Name: "s", NodeOrder: "Alpha,Bravo"})

	rec := jsonRequest(t, NodeBatch, nodeBatchRequest{IDs: []int{a.ID}, Action: "rename", Pattern: "^Alpha$", Replacement: "Alpha-HK"}, nil)
	if rec.Code != 200 {
		t.Fatalf("rename failed: %s", rec.Body.String())
	}
	var renamed models.Node
	models.DB.First(&renamed, a.ID)
	if renamed.Name != "Alpha-HK" || !strings.Contains(renamed.Link, "Alpha-HK") {
		t.Fatalf("name and link must change together: %+v", renamed)
	}
	var sub models.Subcription
	models.DB.Where("name = ?", "s").First(&sub)
	if sub.NodeOrder != "Alpha-HK,Bravo" {
		t.Fatalf("node order not updated: %q", sub.NodeOrder)
	}

	rec = jsonRequest(t, NodeBatch, nodeBatchRequest{IDs: []int{b.ID}, Action: "chain", RelayID: a.ID}, nil)
	if rec.Code != 200 {
		t.Fatalf("chain failed: %s", rec.Body.String())
	}
	var landing models.Node
	models.DB.First(&landing, b.ID)
	// 只输出落地节点时，中转节点会被自动补进配置
	urls, flags := collectNodeInputs([]models.Node{landing})
	if len(urls) != 2 {
		t.Fatalf("relay must be appended, got %v", urls)
	}
	proxies := node.BuildClashProxies(urls, flags, node.SqlConfig{})
	var found bool
	for _, p := range proxies {
		if p.Name == "Bravo" && p.Dialer_proxy == "Alpha-HK" {
			found = true
		}
	}
	if !found {
		t.Fatalf("dialer-proxy missing: %+v", proxies)
	}
}

func TestNodeDedupeMovesReferences(t *testing.T) {
	setupTestDB(t)
	keep := models.Node{Name: "One", Link: testLinkA}
	dup := models.Node{Name: "One copy", Link: strings.Replace(testLinkA, "#Alpha", "#Copy", 1)}
	models.DB.Create(&keep)
	models.DB.Create(&dup)
	sub := models.Subcription{Name: "s"}
	models.DB.Create(&sub)
	_ = models.DB.Model(&sub).Association("Nodes").Append(&dup)

	rec := jsonRequest(t, NodeDedupe, map[string]bool{"preview": false}, nil)
	if rec.Code != 200 {
		t.Fatalf("dedupe failed: %s", rec.Body.String())
	}
	var count int64
	models.DB.Model(&models.Node{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate not removed, %d nodes left", count)
	}
	var reloaded models.Subcription
	models.DB.Preload("Nodes").First(&reloaded, sub.ID)
	if len(reloaded.Nodes) != 1 || reloaded.Nodes[0].ID != keep.ID {
		t.Fatalf("subscription must now reference the kept node: %+v", reloaded.Nodes)
	}
}

func TestTempLinkSignature(t *testing.T) {
	setupTestDB(t)
	token := signTempLink(7, time.Now().Add(time.Hour).Unix())
	id, _, err := parseTempLink(token)
	if err != nil || id != 7 {
		t.Fatalf("valid token rejected: %v", err)
	}
	if _, _, err := parseTempLink(strings.Replace(token, "7.", "8.", 1)); err == nil {
		t.Fatal("tampered token must be rejected")
	}
}

func TestClashProviderMode(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "t.yaml")
	if err := os.WriteFile(tpl, []byte("proxy-groups:\n  - name: Auto\n    type: select\n    proxies: [DIRECT]\nrules:\n  - MATCH,Auto\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := node.EncodeClashProvider(tpl, "sub-abc", "https://x/c/?token=abc&client=clash-proxies", "")
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "proxy-providers:") || !strings.Contains(text, "client=clash-proxies") || !strings.Contains(text, "- sub-abc") {
		t.Fatalf("provider config incomplete:\n%s", text)
	}
}

func TestApplyCountryRenamesNameAndLink(t *testing.T) {
	name, link := applyCountry("Nosla-JP", testLinkA, "jp")
	if name != "🇯🇵 Nosla-JP" || !strings.Contains(link, "Nosla-JP") {
		t.Fatalf("got %q %q", name, link)
	}
	p, err := node.ParseOutbound(link)
	if err != nil || p.Name != "🇯🇵 Nosla-JP" {
		t.Fatalf("link name not updated: %v %+v", err, p)
	}
	if name, _ := applyCountry("🇭🇰 Nosla-JP", testLinkA, ""); name != "Nosla-JP" {
		t.Fatalf("empty country must strip the flag: %q", name)
	}
	if name, _ := applyCountry("lighelayer_USA", testLinkA, "auto"); name != "🇺🇸 lighelayer_USA" {
		t.Fatalf("auto must detect from the name: %q", name)
	}
	if name, _ := applyCountry("Test Reality", testLinkA, "auto"); name != "Test Reality" {
		t.Fatalf("auto without a hint must keep the name: %q", name)
	}
}
