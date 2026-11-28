package rulecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"ppeelink/models"
)

// ResolveClashImportSpec 将目录中的 format / behavior 映射为 Mihomo/Clash rule-provider 的合法字段。
func ResolveClashImportSpec(format, behavior string) (providerFormat, providerBehavior string) {
	f := strings.ToLower(strings.TrimSpace(format))
	b := strings.ToLower(strings.TrimSpace(behavior))
	switch b {
	case "domain", "ipcidr", "classical":
		providerBehavior = b
	default:
		providerBehavior = "classical"
	}
	if f == "yaml" || f == "yml" {
		providerFormat = "yaml"
	} else {
		providerFormat = "text"
	}
	return providerFormat, providerBehavior
}

// ---- 1. Sukka Ruleset (https://ruleset.skk.moe) ----

var sukkaPathRE = regexp.MustCompile(`(Clash|List)/(domainset|non_ip|ip)/([A-Za-z0-9_.-]+)\.(txt|conf)`)

func syncSukkaRuleset(ctx context.Context) ([]RuleItem, error) {
	body, _, err := get(ctx, "https://ruleset.skk.moe/", 4<<20)
	if err != nil {
		return nil, err
	}
	matches := sukkaPathRE.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		return nil, errors.New("未在 ruleset.skk.moe 页面解析到规则链接")
	}
	seen := map[string]bool{}
	items := make([]RuleItem, 0, len(matches))
	for _, m := range matches {
		rootDir, kind, base, ext := m[1], m[2], m[3], m[4]
		platform := "Clash"
		if rootDir == "List" {
			platform = "Surge"
		}
		behavior := "classical"
		switch kind {
		case "domainset":
			behavior = "domain"
		case "ip":
			if base == "china_ip" || base == "china_ip_ipv6" {
				behavior = "ipcidr"
			}
		}
		displayName := base
		if kind == "ip" {
			displayName = base + "_ip"
		} else if kind == "domainset" {
			displayName = base + "_domain"
		}
		relPath := fmt.Sprintf("%s/%s/%s.%s", rootDir, kind, base, ext)
		extID := "sukka_ruleset:" + platform + ":" + kind + ":" + base
		if seen[extID] {
			continue
		}
		seen[extID] = true
		format := "list"
		if ext == "txt" {
			format = "txt"
		}
		items = append(items, RuleItem{
			ExternalID: extID,
			SourceKey:  "sukka_ruleset",
			Name:       displayName,
			Category:   CategoryFor(base),
			Platform:   platform,
			Format:     format,
			Behavior:   behavior,
			URL:        "https://ruleset.skk.moe/" + relPath,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Platform == items[j].Platform {
			return items[i].Name < items[j].Name
		}
		return items[i].Platform < items[j].Platform
	})
	return items, nil
}

// ---- 2. MetaCubeX Official Rules (MetaCubeX/meta-rules-dat@meta) ----

func syncMetaCubeXRules(ctx context.Context) ([]RuleItem, error) {
	items := make([]RuleItem, 0, 512)
	type metaDir struct {
		path     string
		prefix   string
		behavior string
	}
	dirs := []metaDir{
		{path: "geo/geosite", prefix: "geosite:", behavior: "domain"},
		{path: "geo/geoip", prefix: "geoip:", behavior: "ipcidr"},
	}
	for _, d := range dirs {
		apiURL := fmt.Sprintf("https://api.github.com/repos/MetaCubeX/meta-rules-dat/contents/%s?ref=meta", d.path)
		body, _, err := get(ctx, apiURL, 8<<20)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.path, err)
		}
		var entries []ghContent
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Type != "file" || !strings.HasSuffix(strings.ToLower(e.Name), ".yaml") {
				continue
			}
			base := strings.TrimSuffix(e.Name, filepath.Ext(e.Name))
			name := d.prefix + base
			rawURL := fmt.Sprintf("https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/%s/%s", d.path, url.PathEscape(e.Name))
			items = append(items, RuleItem{
				ExternalID:     "metacubex_rules:Clash:" + name,
				SourceKey:      "metacubex_rules",
				Name:           name,
				Category:       CategoryFor(base),
				Platform:       "Clash",
				Format:         "yaml",
				Behavior:       d.behavior,
				URL:            rawURL,
				RemoteRevision: e.SHA,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// ---- 3. Loyalsoldier Curated Clash Rules (Loyalsoldier/clash-rules@release) ----

func syncLoyalsoldierRules(ctx context.Context) ([]RuleItem, error) {
	apiURL := "https://api.github.com/repos/Loyalsoldier/clash-rules/contents?ref=release"
	body, _, err := get(ctx, apiURL, 4<<20)
	if err != nil {
		return nil, err
	}
	var entries []ghContent
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	items := make([]RuleItem, 0, len(entries))
	for _, e := range entries {
		if e.Type != "file" || !strings.HasSuffix(strings.ToLower(e.Name), ".txt") {
			continue
		}
		base := strings.TrimSuffix(e.Name, filepath.Ext(e.Name))
		behavior := "domain"
		switch strings.ToLower(base) {
		case "cncidr", "lancidr", "telegramcidr":
			behavior = "ipcidr"
		case "applications":
			behavior = "classical"
		}
		// Loyalsoldier release .txt files contain YAML `payload:` lists.
		items = append(items, RuleItem{
			ExternalID:     "loyalsoldier_rules:Clash:" + base,
			SourceKey:      "loyalsoldier_rules",
			Name:           base,
			Category:       CategoryFor(base),
			Platform:       "Clash",
			Format:         "yaml",
			Behavior:       behavior,
			URL:            fmt.Sprintf("https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/%s", url.PathEscape(e.Name)),
			RemoteRevision: e.SHA,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// ---- 4. ACL4SSR Ruleset (ACL4SSR/ACL4SSR@master) ----

func syncACL4SSRRules(ctx context.Context) ([]RuleItem, error) {
	seen := map[string]bool{}
	items := make([]RuleItem, 0, 200)
	for _, dir := range []string{"Clash", "Clash/Ruleset"} {
		apiURL := fmt.Sprintf("https://api.github.com/repos/ACL4SSR/ACL4SSR/contents/%s?ref=master", dir)
		body, _, err := get(ctx, apiURL, 8<<20)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		var entries []ghContent
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Type != "file" || !strings.HasSuffix(strings.ToLower(e.Name), ".list") {
				continue
			}
			base := strings.TrimSuffix(e.Name, filepath.Ext(e.Name))
			extID := "acl4ssr_rules:Clash:" + base
			if seen[extID] {
				continue
			}
			seen[extID] = true
			items = append(items, RuleItem{
				ExternalID:     extID,
				SourceKey:      "acl4ssr_rules",
				Name:           base,
				Category:       CategoryFor(base),
				Platform:       "Clash",
				Format:         "list",
				Behavior:       "classical",
				URL:            fmt.Sprintf("https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/%s/%s", dir, url.PathEscape(e.Name)),
				RemoteRevision: e.SHA,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// ---- 5. Cats-Team AdRules (Cats-Team/AdRules@main) ----

func syncCatsAdRules(ctx context.Context) ([]RuleItem, error) {
	apiURL := "https://api.github.com/repos/Cats-Team/AdRules/contents?ref=main"
	body, _, err := get(ctx, apiURL, 2<<20)
	if err != nil {
		return nil, err
	}
	var entries []ghContent
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	shaByName := map[string]string{}
	for _, e := range entries {
		shaByName[e.Name] = e.SHA
	}
	specs := []struct {
		file     string
		name     string
		platform string
		format   string
		behavior string
	}{
		{"adrules.list", "AdRules", "Clash", "list", "classical"},
		{"adrules_domainset.txt", "AdRules-DomainSet", "Clash", "txt", "domain"},
		{"adrules-surge.conf", "AdRules-Surge", "Surge", "list", "classical"},
		{"adrules_surge_domainset.txt", "AdRules-DomainSet", "Surge", "txt", "domain"},
		{"adrules.list", "AdRules", "Loon", "list", "classical"},
	}
	items := make([]RuleItem, 0, len(specs))
	for _, sp := range specs {
		sha, ok := shaByName[sp.file]
		if !ok {
			continue
		}
		items = append(items, RuleItem{
			ExternalID:     fmt.Sprintf("cats_adrules:%s:%s", sp.platform, sp.name),
			SourceKey:      "cats_adrules",
			Name:           sp.name,
			Category:       "广告/隐私",
			Platform:       sp.platform,
			Format:         sp.format,
			Behavior:       sp.behavior,
			URL:            "https://raw.githubusercontent.com/Cats-Team/AdRules/main/" + sp.file,
			RemoteRevision: sha,
		})
	}
	return items, nil
}

// ---- 6. Custom GitHub Repository Rule Source ----

type ghTreeResp struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
		SHA  string `json:"sha"`
	} `json:"tree"`
}

func syncCustomGitHubSource(ctx context.Context, src models.RuleSource) ([]RuleItem, error) {
	repo := strings.Trim(strings.TrimSpace(src.Repo), "/")
	if !strings.Contains(repo, "/") {
		return nil, errors.New("GitHub 仓库格式需为 owner/repo")
	}
	branch := strings.TrimSpace(src.Branch)
	if branch == "" {
		branch = "main"
	}
	prefix := strings.Trim(strings.TrimSpace(src.PathPrefix), "/")
	platform := strings.TrimSpace(src.Platform)
	if platform == "" {
		platform = "Clash"
	}
	behavior := strings.ToLower(strings.TrimSpace(src.Behavior))
	if behavior != "domain" && behavior != "ipcidr" && behavior != "classical" {
		behavior = "classical"
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", repo, url.PathEscape(branch))
	body, _, err := get(ctx, apiURL, 8<<20)
	if err != nil {
		return nil, err
	}
	var tree ghTreeResp
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, err
	}

	items := make([]RuleItem, 0, 64)
	seen := map[string]bool{}
	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}
		p := strings.TrimPrefix(entry.Path, "/")
		if prefix != "" && p != prefix && !strings.HasPrefix(p, prefix+"/") {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(p), "."))
		switch ext {
		case "yaml", "yml", "list", "txt", "conf", "lsr":
		default:
			continue
		}
		if ext == "lsr" || ext == "conf" {
			ext = "list"
		}
		base := strings.TrimSuffix(path.Base(p), path.Ext(p))
		if base == "" || strings.EqualFold(base, "readme") {
			continue
		}
		extID := fmt.Sprintf("%s:%s:%s", src.Key, platform, p)
		if seen[extID] {
			continue
		}
		seen[extID] = true

		escapedParts := strings.Split(p, "/")
		for i := range escapedParts {
			escapedParts[i] = url.PathEscape(escapedParts[i])
		}
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, url.PathEscape(branch), strings.Join(escapedParts, "/"))
		items = append(items, RuleItem{
			ExternalID:     extID,
			SourceKey:      src.Key,
			Name:           base,
			Category:       CategoryFor(base),
			Platform:       platform,
			Format:         ext,
			Behavior:       behavior,
			URL:            rawURL,
			RemoteRevision: entry.SHA,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// CreateCustomSource 创建用户自定义的 GitHub 规则源。
func CreateCustomSource(name, repo, branch, pathPrefix, platform, behavior string) (*models.RuleSource, error) {
	name = strings.TrimSpace(name)
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimPrefix(repo, "http://github.com/")
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.Trim(repo, "/")
	if name == "" || strings.Count(repo, "/") != 1 {
		return nil, errors.New("请填写名称与合法的 GitHub 仓库（例如 owner/repo）")
	}
	if branch = strings.TrimSpace(branch); branch == "" {
		branch = "main"
	}
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "surge":
		platform = "Surge"
	case "loon":
		platform = "Loon"
	default:
		platform = "Clash"
	}
	switch strings.ToLower(strings.TrimSpace(behavior)) {
	case "domain", "ipcidr", "classical":
		behavior = strings.ToLower(strings.TrimSpace(behavior))
	default:
		behavior = "classical"
	}
	slug := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(repo+"_"+pathPrefix), "_")
	slug = strings.Trim(slug, "_")
	if len(slug) > 48 {
		slug = slug[:48]
	}
	key := "custom_" + slug
	var existing models.RuleSource
	if models.DB.Where("key = ?", key).First(&existing).Error == nil {
		return nil, errors.New("该规则源已存在")
	}
	src := models.RuleSource{
		Key:        key,
		Name:       name,
		Type:       "github-custom",
		Repo:       repo,
		Branch:     branch,
		BaseURL:    fmt.Sprintf("https://raw.githubusercontent.com/%s/%s", repo, branch),
		PathPrefix: strings.Trim(strings.TrimSpace(pathPrefix), "/"),
		Platform:   platform,
		Behavior:   behavior,
		Custom:     true,
		Enabled:    true,
	}
	if err := models.DB.Create(&src).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

// DeleteCustomSource 删除用户自定义的规则源及其目录缓存记录。
func DeleteCustomSource(key string) error {
	var src models.RuleSource
	if err := models.DB.Where("key = ?", key).First(&src).Error; err != nil {
		return errors.New("规则源不存在")
	}
	if !src.Custom {
		return errors.New("内置规则源不支持删除")
	}
	if err := models.DB.Where("source_key = ?", key).Delete(&models.RuleCatalog{}).Error; err != nil {
		return err
	}
	return models.DB.Delete(&src).Error
}
