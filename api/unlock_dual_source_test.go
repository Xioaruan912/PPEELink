package api

import (
	"encoding/json"
	"strings"
	"testing"

	"ppeelink/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDualUnlockTestDB(t *testing.T) func() {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ProxyServer{}, &models.Node{}, &models.ClashUnlockSnapshot{}, &models.TemplateVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := models.DB
	oldDir := baseTemplateDir
	models.DB = db
	baseTemplateDir = t.TempDir()
	return func() {
		models.DB = oldDB
		baseTemplateDir = oldDir
	}
}

func TestDualSourceUnlockMergeAndClashPriority(t *testing.T) {
	cleanup := setupDualUnlockTestDB(t)
	defer cleanup()

	// Seed 2 self-hosted servers:
	// 1) DMIT_USA: has BOTH native + Clash (ChatGPT agrees, Disney conflicts: native available vs Clash blocked -> Clash wins!)
	// 2) NoSLA_JP: native ONLY (no Clash snapshot)
	// Plus 1 standalone third-party/relay Clash node: "🇯🇵 沪日专线-JP" (Clash ONLY, no ProxyServer)
	// Plus 1 broken CN-exit Clash item that MUST be ignored!
	dmitNative, _ := json.Marshal(serverOpsUnlockReport{
		IP:      "1.1.1.1",
		Country: "US",
		Items: []serverOpsUnlockItem{
			{Key: "chatgpt", Name: "ChatGPT", Status: "available", Region: "US"},
			{Key: "claude", Name: "Claude", Status: "available", Region: "US"},
			{Key: "gemini", Name: "Gemini", Status: "available", Region: "US"},
			{Key: "disney", Name: "Disney+", Status: "available", Region: "US"},
			{Key: "tiktok", Name: "TikTok", Status: "available", Region: "US"}, // native-only service
		},
	})
	jpNative, _ := json.Marshal(serverOpsUnlockReport{
		IP:      "2.2.2.2",
		Country: "JP",
		Items: []serverOpsUnlockItem{
			{Key: "chatgpt", Name: "ChatGPT", Status: "available", Region: "JP"},
			{Key: "claude", Name: "Claude", Status: "available", Region: "JP"},
			{Key: "disney", Name: "Disney+", Status: "available", Region: "JP"},
		},
	})
	srvUS := models.ProxyServer{Name: "DMIT_USA", Kind: models.ServerKindAgent, PublicHost: "1.1.1.1", Enabled: true, UnlockJSON: string(dmitNative)}
	srvJP := models.ProxyServer{Name: "NoSLA_JP", Kind: models.ServerKindAgent, PublicHost: "2.2.2.2", Enabled: true, UnlockJSON: string(jpNative)}
	models.DB.Create(&srvUS)
	models.DB.Create(&srvJP)

	saved := saveClashBatchUnlockSnapshots("福州", []clashNodeTestResultDTO{
		{
			Name:         "🇺🇸 DMIT_USA",
			EgressIP:     "1.1.1.1",
			Country:      "US",
			TTFBMs:       220,
			DownloadMbps: 145.6,
			ChatGPT:      "available:US",
			Claude:       "available:US",
			Gemini:       "blocked", // Conflict: native available vs Clash blocked -> Clash wins!
			Netflix:      "available:US",
			Disney:       "blocked", // Conflict: native available vs Clash blocked -> Clash wins!
			YouTube:      "available:US",
		},
		{
			Name:         "🇯🇵 沪日专线-JP",
			EgressIP:     "3.3.3.3",
			Country:      "JP",
			TTFBMs:       95,
			DownloadMbps: 310.2,
			ChatGPT:      "available:JP",
			Claude:       "blocked",
			Gemini:       "available:JP",
			Netflix:      "available:JP",
			Disney:       "available:JP",
			YouTube:      "available:JP",
		},
		{
			// Broken CN direct leak must NEVER be persisted.
			Name:    "🇭🇰 AsiaCLoud_HK",
			Country: "CN",
			Netflix: "blocked",
			Disney:  "blocked",
		},
	})
	if saved != 2 {
		t.Fatalf("expected 2 valid Clash snapshots saved (CN skipped), got %d", saved)
	}

	// Verify merged report for DMIT_USA (both sources)
	clashByServer, _ := loadClashSnapshotsByServerID()
	mergedUS := buildMergedServerUnlockReport(&srvUS, clashByServer)
	if mergedUS == nil || len(mergedUS.Sources) != 2 {
		t.Fatalf("expected DMIT_USA to have dual sources, got %+v", mergedUS)
	}
	byKey := map[string]serverOpsUnlockItem{}
	for _, it := range mergedUS.Items {
		byKey[it.Key] = it
	}
	if byKey["chatgpt"].Source != "both" || byKey["chatgpt"].Consensus != "agree" || byKey["chatgpt"].Status != "available" {
		t.Fatalf("unexpected DMIT_USA chatgpt merge: %+v", byKey["chatgpt"])
	}
	if byKey["disney"].Source != "both" || byKey["disney"].Consensus != "conflict" || byKey["disney"].Status != "blocked" || byKey["disney"].NativeStatus != "available" || byKey["disney"].ClashStatus != "blocked" {
		t.Fatalf("expected Clash priority on disney conflict, got %+v", byKey["disney"])
	}
	if byKey["tiktok"].Source != "native" || byKey["tiktok"].Status != "available" {
		t.Fatalf("expected native-only tiktok preserved, got %+v", byKey["tiktok"])
	}
	if byKey["netflix"].Source != "clash" || byKey["netflix"].Status != "available" {
		t.Fatalf("expected clash-only netflix added, got %+v", byKey["netflix"])
	}

	// Verify NoSLA_JP stays pure native
	mergedJP := buildMergedServerUnlockReport(&srvJP, clashByServer)
	if mergedJP == nil || len(mergedJP.Sources) != 1 || mergedJP.Sources[0] != "native" {
		t.Fatalf("expected NoSLA_JP native-only, got %+v", mergedJP)
	}

	// Verify unified smart routing filter includes standalone Clash node "沪日专线-JP" AND respects Clash conflict priority!
	gptFilter, ok := resolveSmartUnlockProviderFilter("ChatGPT", "")
	if !ok || !strings.Contains(gptFilter, "DMIT_USA") || !strings.Contains(gptFilter, "NoSLA_JP") || !strings.Contains(gptFilter, "沪日专线-JP") {
		t.Fatalf("expected ChatGPT filter to include all 3 unlocked entities, got %q", gptFilter)
	}
	disneyFilter, ok := resolveSmartUnlockProviderFilter("Disney+", "")
	if !ok || strings.Contains(disneyFilter, "DMIT_USA") || !strings.Contains(disneyFilter, "NoSLA_JP") || !strings.Contains(disneyFilter, "沪日专线-JP") {
		t.Fatalf("expected Disney+ filter to exclude DMIT_USA (Clash blocked wins conflict) and include NoSLA_JP + 沪日专线-JP, got %q", disneyFilter)
	}
	geminiFilter, ok := resolveSmartUnlockProviderFilter("Gemini", "")
	if !ok || strings.Contains(geminiFilter, "DMIT_USA") || !strings.Contains(geminiFilter, "沪日专线-JP") {
		t.Fatalf("expected Gemini filter to exclude DMIT_USA (Clash blocked) and include 沪日专线-JP, got %q", geminiFilter)
	}
}
