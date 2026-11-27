package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ppeelink/models"
	"ppeelink/node"
)

func TestBuildSmartLoonAndSurgeTemplatesLiveLinkage(t *testing.T) {
	setupTestDB(t)
	_ = models.DB.AutoMigrate(&models.ProxyServer{}, &models.ClashUnlockSnapshot{})
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	report := `{"items":[{"key":"chatgpt","name":"ChatGPT","status":"available"},{"key":"claude","name":"Claude","status":"available"},{"key":"gemini","name":"Gemini","status":"available"},{"key":"netflix","name":"Netflix","status":"available"},{"key":"disney","name":"Disney+","status":"blocked"},{"key":"tiktok","name":"TikTok","status":"available"}]}`
	if err := models.DB.Create(&models.ProxyServer{
		Name:       "DMIT_USA",
		Kind:       models.ServerKindAgent,
		Enabled:    true,
		PublicHost: "1.2.3.4",
		UnlockJSON: report,
	}).Error; err != nil {
		t.Fatal(err)
	}

	loonText, _, checked, total := BuildSmartLoonTemplate()
	if checked != 1 || total != 1 {
		t.Fatalf("expected 1/1 server, got %d/%d", checked, total)
	}
	if !strings.Contains(loonText, "ChatGPT解锁 = NameRegex, FilterKey =") || !strings.Contains(loonText, "DMIT_USA") {
		t.Fatalf("expected DMIT_USA in Loon smart filter, got:\n%s", loonText)
	}

	surgeText, _, _, _ := BuildSmartSurgeTemplate()
	if !strings.Contains(surgeText, "ChatGPT = select, 日常使用, filter(") || !strings.Contains(surgeText, "DMIT_USA") {
		t.Fatalf("expected DMIT_USA in Surge smart filter, got:\n%s", surgeText)
	}

	// Verify DecodeSurge expands filter(...) properly without duplicating sections
	surgePath := filepath.Join(tmpDir, "surge-smart.conf")
	if err := os.WriteFile(surgePath, []byte(surgeText), 0644); err != nil {
		t.Fatal(err)
	}
	rendered, err := node.DecodeSurge(
		[]string{"🇺🇸 DMIT_USA = vmess, 1.2.3.4, 443", "🇭🇰 HK_Node = vmess, 2.3.4.5, 443"},
		[]string{"🇺🇸 DMIT_USA", "🇭🇰 HK_Node"},
		surgePath,
	)
	if err != nil {
		t.Fatalf("DecodeSurge failed: %v", err)
	}
	if strings.Contains(rendered, "filter(") {
		t.Fatalf("expected filter(...) markers removed in rendered Surge config")
	}
	if !strings.Contains(rendered, "ChatGPT = select, 日常使用, 🇺🇸 DMIT_USA") {
		t.Fatalf("expected ChatGPT group to include matched DMIT_USA node, got:\n%s", rendered)
	}
	if strings.Count(rendered, "[Proxy Group]") != 1 {
		t.Fatalf("expected exactly 1 [Proxy Group] header, got %d", strings.Count(rendered, "[Proxy Group]"))
	}
}
