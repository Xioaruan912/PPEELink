package rulecenter

import (
	"testing"

	"ppeelink/models"
)

func TestParseRulesYAMLAndMatch(t *testing.T) {
	data := []byte("payload:\n  - DOMAIN-SUFFIX,openai.com\n  - DOMAIN,chatgpt.com\n  - DOMAIN-KEYWORD,anthropic\n")
	rules, warnings, err := ParseRules(data, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(rules) != 3 {
		t.Fatalf("want 3 rules, got %d", len(rules))
	}
	for _, domain := range []string{"openai.com", "api.openai.com", "chatgpt.com", "cdn.anthropic-assets.com"} {
		if !MatchDomain(rules, domain) {
			t.Fatalf("expected match for %s", domain)
		}
	}
	if MatchDomain(rules, "example.com") {
		t.Fatal("example.com should not match")
	}
}

func TestRuleUserAgentForKelee(t *testing.T) {
	if got := ruleUserAgent("https://rule.kelee.one/Clash/OpenAI.yaml"); got != "clash.meta" {
		t.Fatalf("unexpected kelee user-agent: %s", got)
	}
	if got := ruleUserAgent("https://raw.githubusercontent.com/a/b/main/x.yaml"); got == "clash.meta" {
		t.Fatalf("non-kelee URL should keep the generic user-agent")
	}
}

func TestParseRulesUnknownTypeWarning(t *testing.T) {
	rules, warnings, err := ParseRules([]byte("FOO,bar.example\nDOMAIN-SUFFIX,example.com\n"), "list")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("want 2 normalized rows, got %d", len(rules))
	}
	if len(warnings) != 1 {
		t.Fatalf("want one warning, got %v", warnings)
	}
}

func TestParseDomainBehavior(t *testing.T) {
	rules, warnings, err := ParseRulesWithBehavior([]byte("payload:\n  - +.google.com\n  - gemini.google.com\n  - '*.example.net'\n"), "yaml", "domain")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(rules) != 3 {
		t.Fatalf("unexpected domain provider result: %#v, warnings=%v", rules, warnings)
	}
	for _, domain := range []string{"gemini.google.com", "api.google.com", "cdn.example.net"} {
		if !MatchDomain(rules, domain) {
			t.Fatalf("domain provider did not match %q: %#v", domain, rules)
		}
	}
}

func TestParseIPCIDRBehavior(t *testing.T) {
	rules, warnings, err := ParseRulesWithBehavior([]byte("1.1.1.0/24\n2001:db8::/32\n"), "text", "ipcidr")
	if err != nil || len(warnings) != 0 || len(rules) != 2 {
		t.Fatalf("unexpected ipcidr provider result: rules=%#v warnings=%v err=%v", rules, warnings, err)
	}
	if rules[0].Type != "IP-CIDR" || rules[1].Type != "IP-CIDR6" {
		t.Fatalf("unexpected ipcidr types: %#v", rules)
	}
}

func TestNormalizedContentHashAndDeduplicateCatalog(t *testing.T) {
	yamlRules, _, err := ParseRules([]byte("# header A\npayload:\n  - DOMAIN,b.example.com\n  - DOMAIN-SUFFIX,a.example.com\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	listRules, _, err := ParseRules([]byte("# header B\nDOMAIN-SUFFIX, a.example.com\nDOMAIN, b.example.com\n"), "list")
	if err != nil {
		t.Fatal(err)
	}
	diffRules, _, err := ParseRules([]byte("DOMAIN-SUFFIX, a.example.com\nDOMAIN, c.example.com\n"), "list")
	if err != nil {
		t.Fatal(err)
	}

	hYAML := NormalizedContentHash(yamlRules)
	hList := NormalizedContentHash(listRules)
	hDiff := NormalizedContentHash(diffRules)
	if hYAML == "" || hYAML != hList {
		t.Fatalf("expected identical normalized hash across format/order/spacing, got yaml=%q list=%q", hYAML, hList)
	}
	if hYAML == hDiff {
		t.Fatalf("expected different hash for different rule content, got %q", hDiff)
	}

	records := []models.RuleCatalog{
		{ID: 1, SourceKey: "shunt_rules", Platform: "Loon", Name: "OpenAI", RuleCount: 2, ContentHash: hList},
		{ID: 2, SourceKey: "ios_rule_script", Platform: "Clash", Name: "OpenAI", RuleCount: 2, ContentHash: hYAML},
		{ID: 3, SourceKey: "shunt_rules", Platform: "Clash", Name: "OpenAI", RuleCount: 2, ContentHash: hYAML},
		{ID: 4, SourceKey: "ios_rule_script", Platform: "Clash", Name: "OpenAI-Alt", RuleCount: 2, ContentHash: hDiff},
		{ID: 5, SourceKey: "shunt_rules", Platform: "Clash", Name: "UncachedA", RuleCount: 0, ContentHash: ""},
		{ID: 6, SourceKey: "ios_rule_script", Platform: "Clash", Name: "UncachedB", RuleCount: 0, ContentHash: ""},
	}
	got := deduplicateCatalog(records)
	if len(got) != 4 {
		t.Fatalf("want 4 deduplicated items, got %d: %#v", len(got), got)
	}
	if got[0].ID != 2 || got[0].Platform != "Clash" || got[0].SourceKey != "ios_rule_script" || got[0].DuplicateCount != 2 {
		t.Fatalf("expected preferred Clash/ios_rule_script representative with DuplicateCount=2, got %#v", got[0])
	}
	if got[1].ID != 4 || got[1].DuplicateCount != 0 {
		t.Fatalf("expected distinct-content item to be kept untouched, got %#v", got[1])
	}
}
