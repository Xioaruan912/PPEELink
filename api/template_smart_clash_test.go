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

	"ppeelink/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSmartClashTestDB(t *testing.T) func() {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ProxyServer{}, &models.Node{}, &models.Subcription{}, &models.TemplateVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := models.DB
	oldDir := baseTemplateDir
	tmpDir := t.TempDir()
	models.DB = db
	baseTemplateDir = tmpDir
	return func() {
		models.DB = oldDB
		baseTemplateDir = oldDir
	}
}

func TestSmartClashTemplateDefaultAndLiveLinkage(t *testing.T) {
	cleanup := setupSmartClashTestDB(t)
	defer cleanup()

	// 1. Without any unlock data, default template builds cleanly and passes validator.
	yamlText, summaries, checked, total, err := BuildSmartClashTemplateYAML()
	if err != nil {
		t.Fatalf("BuildSmartClashTemplateYAML failed: %v", err)
	}
	if checked != 0 || total != 0 {
		t.Fatalf("expected 0/0 servers, got %d/%d", checked, total)
	}
	if len(summaries) != 6 {
		t.Fatalf("expected 6 smart service groups, got %d", len(summaries))
	}
	_, errs := validateTemplateText(SmartClashDefaultFilename, yamlText)
	if len(errs) > 0 {
		t.Fatalf("template validation errors: %v", errs)
	}
	for _, mustContain := range []string{"name: 日常使用", "name: ChatGPT", "name: Claude", "name: Gemini", "name: Netflix", "name: Disney+", "name: TikTok", "RULE-SET,Advertising,REJECT", "RULE-SET,OpenAI,ChatGPT", "RULE-SET,Netflix,Netflix", "MATCH,日常使用"} {
		if !strings.Contains(yamlText, mustContain) {
			t.Fatalf("expected generated YAML to contain %q", mustContain)
		}
	}

	// Verify Advertising and OpenAI come BEFORE Proxy rule.
	if strings.Index(yamlText, "RULE-SET,OpenAI,ChatGPT") > strings.Index(yamlText, "RULE-SET,Proxy,日常使用") {
		t.Fatalf("expected OpenAI rule before Proxy rule")
	}

	// 2. Seed servers with native unlock reports and verify live filter linkage + TikTok empty fallback.
	usReport, _ := json.Marshal(serverOpsUnlockReport{
		Items: []serverOpsUnlockItem{
			{Key: "chatgpt", Status: "available", Region: "US"},
			{Key: "claude", Status: "available", Region: "US"},
			{Key: "gemini", Status: "available", Region: "US"},
			{Key: "netflix", Status: "available", Region: "US"},
			{Key: "disney", Status: "available", Region: "US"},
			{Key: "tiktok", Status: "blocked"},
		},
	})
	hkReport, _ := json.Marshal(serverOpsUnlockReport{
		Items: []serverOpsUnlockItem{
			{Key: "chatgpt", Status: "blocked"},
			{Key: "claude", Status: "blocked"},
			{Key: "gemini", Status: "blocked"},
			{Key: "netflix", Status: "available", Region: "HK"},
			{Key: "disney", Status: "available", Region: "HK"},
			{Key: "tiktok", Status: "blocked"},
		},
	})
	models.DB.Create(&models.ProxyServer{Name: "DMIT_USA", Kind: models.ServerKindAgent, PublicHost: "1.1.1.1", Enabled: true, UnlockJSON: string(usReport)})
	models.DB.Create(&models.ProxyServer{Name: "AsiaCLoud_HK", Kind: models.ServerKindAgent, PublicHost: "2.2.2.2", Enabled: true, UnlockJSON: string(hkReport)})

	liveYAML, liveSummaries, checked2, total2, err := BuildSmartClashTemplateYAML()
	if err != nil {
		t.Fatalf("BuildSmartClashTemplateYAML live failed: %v", err)
	}
	if checked2 != 2 || total2 != 2 {
		t.Fatalf("expected 2/2 servers, got %d/%d", checked2, total2)
	}
	_, liveErrs := validateTemplateText(SmartClashDefaultFilename, liveYAML)
	if len(liveErrs) > 0 {
		t.Fatalf("live template validation errors: %v", liveErrs)
	}

	byName := map[string]SmartClashGroupSummary{}
	for _, s := range liveSummaries {
		byName[s.Name] = s
	}
	if !byName["ChatGPT"].LiveMatched || !strings.Contains(byName["ChatGPT"].Filter, "DMIT_USA") || strings.Contains(byName["ChatGPT"].Filter, "AsiaCLoud_HK") {
		t.Fatalf("unexpected ChatGPT summary: %+v", byName["ChatGPT"])
	}
	if !byName["Netflix"].LiveMatched || !strings.Contains(byName["Netflix"].Filter, "DMIT_USA") || !strings.Contains(byName["Netflix"].Filter, "AsiaCLoud_HK") {
		t.Fatalf("unexpected Netflix summary: %+v", byName["Netflix"])
	}
	if byName["TikTok"].LiveMatched || !byName["TikTok"].Fallback || byName["TikTok"].Filter != "" {
		t.Fatalf("expected TikTok to cleanly fallback when 0 servers unlock it, got %+v", byName["TikTok"])
	}
	if !strings.Contains(liveYAML, "- 日常使用") {
		t.Fatalf("expected TikTok fallback group to reference 日常使用 in YAML")
	}

	// 3. Test HTTP endpoint GenerateSmartClashTemplate with applyAllSubs=true preserving proxyProvider.
	models.DB.Create(&models.Subcription{
		Name:   "main-sub",
		Config: `{"clash":"./template/clash.yaml","surge":"./template/surge.conf","udp":true,"cert":false,"proxyProvider":true}`,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/template/smart-clash/generate", GenerateSmartClashTemplate)

	body := bytes.NewBufferString(`{"applyAllSubs":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/template/smart-clash/generate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(baseTemplateDir, SmartClashDefaultFilename)); err != nil {
		t.Fatalf("expected clash-smart.yaml written to disk: %v", err)
	}
	var updated models.Subcription
	if err := models.DB.Where("name = ?", "main-sub").First(&updated).Error; err != nil {
		t.Fatalf("query sub: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(updated.Config), &cfg); err != nil {
		t.Fatalf("unmarshal sub config: %v", err)
	}
	if cfg["clash"] != "./template/clash-smart.yaml" {
		t.Fatalf("expected clash template updated, got %v", cfg["clash"])
	}
	if cfg["proxyProvider"] != true || cfg["udp"] != true {
		t.Fatalf("expected existing subscription flags preserved, got %+v", cfg)
	}
}
