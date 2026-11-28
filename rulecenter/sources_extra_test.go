package rulecenter

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ppeelink/models"
)

func TestResolveClashImportSpecAndBehaviorParsing(t *testing.T) {
	f, b := ResolveClashImportSpec("yaml", "domain")
	if f != "yaml" || b != "domain" {
		t.Fatalf("unexpected yaml/domain spec: %s/%s", f, b)
	}
	f, b = ResolveClashImportSpec("list", "classical")
	if f != "text" || b != "classical" {
		t.Fatalf("unexpected list/classical spec: %s/%s", f, b)
	}
	f, b = ResolveClashImportSpec("txt", "ipcidr")
	if f != "text" || b != "ipcidr" {
		t.Fatalf("unexpected txt/ipcidr spec: %s/%s", f, b)
	}

	// Loyalsoldier YAML domain payload
	loyalDomain := []byte("payload:\n  - '+.openai.com'\n  - 'chatgpt.com'\n")
	rules, _, err := ParseRulesWithBehavior(loyalDomain, "yaml", "domain")
	if err != nil || len(rules) != 2 || rules[0].Type != "DOMAIN-SUFFIX" || rules[1].Type != "DOMAIN" {
		t.Fatalf("failed parsing Loyalsoldier domain YAML: err=%v rules=%+v", err, rules)
	}

	// Sukka text bare IP-CIDR payload
	sukkaIP := []byte("# comment\n1.1.8.0/24\n2001:250::/30\n")
	ipRules, _, err := ParseRulesWithBehavior(sukkaIP, "txt", "ipcidr")
	if err != nil || len(ipRules) != 2 || ipRules[0].Type != "IP-CIDR" || ipRules[1].Type != "IP-CIDR6" {
		t.Fatalf("failed parsing Sukka ipcidr txt: err=%v rules=%+v", err, ipRules)
	}

	// ImportClashProvider uses text + .list path for text providers
	baseTpl := "rule-providers: {}\nrules:\n  - MATCH,Proxy\n"
	res, err := ImportClashProvider(baseTpl, ImportOptions{
		ProviderName: "SukkaAI",
		URL:          "https://ruleset.skk.moe/Clash/non_ip/ai.txt",
		Policy:       "ChatGPT",
		Behavior:     "classical",
		Format:       "text",
	})
	if err != nil {
		t.Fatalf("ImportClashProvider failed: %v", err)
	}
	if !strings.Contains(res.Text, "format: text") || !strings.Contains(res.Text, "./rules/SukkaAI.list") {
		t.Fatalf("expected text format & .list path in imported YAML:\n%s", res.Text)
	}
}

func TestCustomRuleSourceLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.RuleSource{}, &models.RuleCatalog{}); err != nil {
		t.Fatal(err)
	}
	origDB := models.DB
	models.DB = db
	defer func() { models.DB = origDB }()

	if err := EnsureSources(); err != nil {
		t.Fatalf("EnsureSources failed: %v", err)
	}
	srcs, err := Sources()
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 7 {
		t.Fatalf("expected 7 built-in rule sources, got %d", len(srcs))
	}

	custom, err := CreateCustomSource("EverythingRules", "https://github.com/Xioaruan912/EverythingRules.git", "main", "rules/clash", "Clash", "classical")
	if err != nil {
		t.Fatalf("CreateCustomSource failed: %v", err)
	}
	if custom.Repo != "Xioaruan912/EverythingRules" || !custom.Custom {
		t.Fatalf("unexpected custom source: %+v", custom)
	}
	if err := DeleteCustomSource("shunt_rules"); err == nil {
		t.Fatalf("expected error deleting built-in source")
	}
	if err := DeleteCustomSource(custom.Key); err != nil {
		t.Fatalf("DeleteCustomSource failed: %v", err)
	}
}
