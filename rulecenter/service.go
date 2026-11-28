package rulecenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/utils"
)

const cacheRoot = "db/rules-cache"

var httpClient = utils.SafeHTTPClient(12 * time.Second)
var syncMu sync.Mutex
var warmRunning sync.Map

var sourceDefs = []models.RuleSource{
	{Key: "shunt_rules", Name: "ShuntRules", Type: "github-readme", Repo: "luestr/ShuntRules", Branch: "main", BaseURL: "https://rule.kelee.one", Enabled: true},
	{Key: "ios_rule_script", Name: "ios_rule_script", Type: "github-contents", Repo: "blackmatrix7/ios_rule_script", Branch: "master", BaseURL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master", Enabled: true},
	{Key: "sukka_ruleset", Name: "Sukka Ruleset", Type: "sukka-html", Repo: "SukkaW/Surge", Branch: "master", BaseURL: "https://ruleset.skk.moe", Enabled: true},
	{Key: "metacubex_rules", Name: "MetaCubeX 官方", Type: "github-contents", Repo: "MetaCubeX/meta-rules-dat", Branch: "meta", BaseURL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta", Enabled: true},
	{Key: "loyalsoldier_rules", Name: "Loyalsoldier", Type: "github-contents", Repo: "Loyalsoldier/clash-rules", Branch: "release", BaseURL: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release", Enabled: true},
	{Key: "acl4ssr_rules", Name: "ACL4SSR", Type: "github-contents", Repo: "ACL4SSR/ACL4SSR", Branch: "master", BaseURL: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master", Enabled: true},
	{Key: "cats_adrules", Name: "AdRules 去广告", Type: "github-contents", Repo: "Cats-Team/AdRules", Branch: "main", BaseURL: "https://raw.githubusercontent.com/Cats-Team/AdRules/main", Enabled: true},
}

func EnsureSources() error {
	for _, s := range sourceDefs {
		var existing models.RuleSource
		err := models.DB.Where("key = ?", s.Key).First(&existing).Error
		if err == nil {
			continue
		}
		if err := models.DB.Create(&s).Error; err != nil {
			return err
		}
	}
	return nil
}

func SyncAll(ctx context.Context) error {
	syncMu.Lock()
	defer syncMu.Unlock()
	if err := EnsureSources(); err != nil {
		return err
	}
	var srcs []models.RuleSource
	if err := models.DB.Where("enabled = ?", true).Order("id asc").Find(&srcs).Error; err != nil {
		return err
	}
	var errs []string
	for _, src := range srcs {
		if err := SyncSource(ctx, src.Key); err != nil {
			errs = append(errs, src.Key+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func SyncSource(ctx context.Context, key string) error {
	_ = EnsureSources()
	var src models.RuleSource
	if err := models.DB.Where("key = ?", key).First(&src).Error; err != nil {
		return err
	}
	var items []RuleItem
	var err error
	switch key {
	case "shunt_rules":
		items, err = syncShuntRules(ctx)
	case "ios_rule_script":
		items, err = syncIOSRuleScript(ctx)
	case "sukka_ruleset":
		items, err = syncSukkaRuleset(ctx)
	case "metacubex_rules":
		items, err = syncMetaCubeXRules(ctx)
	case "loyalsoldier_rules":
		items, err = syncLoyalsoldierRules(ctx)
	case "acl4ssr_rules":
		items, err = syncACL4SSRRules(ctx)
	case "cats_adrules":
		items, err = syncCatsAdRules(ctx)
	default:
		if src.Type == "github-custom" {
			items, err = syncCustomGitHubSource(ctx, src)
		} else {
			err = fmt.Errorf("unknown rule source: %s", key)
		}
	}
	now := time.Now()
	if err != nil {
		models.DB.Model(&src).Updates(map[string]any{"last_sync_at": &now, "last_sync_status": "error", "last_sync_error": err.Error()})
		return err
	}

	var previous []models.RuleCatalog
	if err := models.DB.Where("source_key = ?", key).Find(&previous).Error; err != nil {
		return err
	}
	oldByID := make(map[string]models.RuleCatalog, len(previous))
	for _, rec := range previous {
		oldByID[rec.ExternalID] = rec
	}

	// Catalog refresh is metadata-only and preserves the verified local cache.
	// RemoteRevision changes mark a cached file stale; unchanged revisions keep
	// the exact cache file/statistics without rewriting anything.
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_key = ?", key).Delete(&models.RuleCatalog{}).Error; err != nil {
			return err
		}
		for _, item := range items {
			meta, _ := json.Marshal(item.Metadata)
			behavior := item.Behavior
			if behavior == "" {
				behavior = "classical"
			}
			rec := models.RuleCatalog{SourceKey: item.SourceKey, ExternalID: item.ExternalID, Name: item.Name, Category: item.Category, Platform: item.Platform, Format: item.Format, Behavior: behavior, URL: item.URL, RemoteUpdate: item.UpdatedAt, RemoteRevision: item.RemoteRevision, MetadataJSON: string(meta)}
			if old, ok := oldByID[item.ExternalID]; ok {
				rec.LocalPath = old.LocalPath
				rec.RuleCount = old.RuleCount
				rec.Checksum = old.Checksum
				rec.ContentHash = old.ContentHash
				rec.MetadataJSON = old.MetadataJSON
				rec.CacheRevision = old.CacheRevision
				if old.RemoteRevision != "" && old.RemoteRevision == item.RemoteRevision && old.CacheRevision == item.RemoteRevision && old.URL != "" {
					rec.URL = old.URL
				}
			}
			if err := tx.Create(&rec).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := models.DB.Model(&src).Updates(map[string]any{"last_sync_at": &now, "last_sync_status": "ok", "last_sync_error": ""}).Error; err != nil {
		return err
	}

	// Cache refresh is intentionally detached from the request lifecycle. It
	// only rewrites a rule file when content/revision actually changed.
	go WarmSourceCache(context.Background(), key)
	return nil
}

func get(ctx context.Context, rawURL string, max int64) ([]byte, http.Header, error) {
	body, header, err := getDirect(ctx, rawURL, max)
	if err == nil {
		return body, header, nil
	}
	proxyBody, proxyHeader, proxyErr := getThroughBestNodes(ctx, rawURL, max)
	if proxyErr == nil {
		return proxyBody, proxyHeader, nil
	}
	return nil, header, fmt.Errorf("直连失败: %v; 节点回退失败: %v", err, proxyErr)
}

func getDirect(ctx context.Context, rawURL string, max int64) ([]byte, http.Header, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, nil, errors.New("invalid rule URL")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	req.Header.Set("User-Agent", ruleUserAgent(rawURL))
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	lr := io.LimitReader(resp.Body, max+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, resp.Header, err
	}
	if int64(len(body)) > max {
		return nil, resp.Header, errors.New("rule file exceeds size limit")
	}
	return body, resp.Header, nil
}

func getThroughBestNodes(ctx context.Context, rawURL string, max int64) ([]byte, http.Header, error) {
	nodes, err := models.GetNodeList()
	if err != nil || len(nodes) == 0 {
		return nil, nil, errors.New("没有可用节点")
	}
	stats, _ := models.GetNodeQualityStats(time.Now().Add(-24 * time.Hour))
	sort.SliceStable(nodes, func(i, j int) bool {
		si, iok := stats[nodes[i].ID]
		sj, jok := stats[nodes[j].ID]
		if iok != jok {
			return iok
		}
		if iok && si.Score != sj.Score {
			return si.Score > sj.Score
		}
		if iok && si.AverageRtt >= 0 && sj.AverageRtt >= 0 && si.AverageRtt != sj.AverageRtt {
			return si.AverageRtt < sj.AverageRtt
		}
		return nodes[i].ID < nodes[j].ID
	})

	// Do not let the highest-scoring nodes all come from one blocked region.
	// First take one strong node per country, then fill the remaining slots by
	// global quality order. This makes a 403 fallback actually geo-diverse.
	const limit = 8
	candidates := make([]models.Node, 0, limit)
	selected := map[int]bool{}
	countries := map[string]bool{}
	for _, candidate := range nodes {
		host, _ := node.ExtractServerHost(candidate.Link)
		country := node.LookupCountry(host)
		if country == "" {
			country = "unknown"
		}
		if countries[country] {
			continue
		}
		countries[country] = true
		selected[candidate.ID] = true
		candidates = append(candidates, candidate)
		if len(candidates) >= limit {
			break
		}
	}
	if len(candidates) < limit {
		for _, candidate := range nodes {
			if selected[candidate.ID] {
				continue
			}
			candidates = append(candidates, candidate)
			if len(candidates) >= limit {
				break
			}
		}
	}

	var errs []string
	for _, candidate := range candidates {
		body, header, fetchErr := node.FetchURLThroughNode(ctx, candidate.Link, rawURL, ruleUserAgent(rawURL), 7*time.Second, max)
		if fetchErr == nil {
			return body, header, nil
		}
		errs = append(errs, candidate.Name+": "+fetchErr.Error())
		if ctx.Err() != nil {
			break
		}
	}
	return nil, nil, errors.New(strings.Join(errs, "; "))
}

func ruleUserAgent(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		host := strings.ToLower(u.Hostname())
		if host == "kelee.one" || strings.HasSuffix(host, ".kelee.one") {
			return "clash.meta"
		}
	}
	return "SubLinkX-RuleCenter/1.0"
}

var shuntLinkRE = regexp.MustCompile(`https://rule\.kelee\.one/(Clash|Loon)/([^\s)]+)`)

func syncShuntRules(ctx context.Context) ([]RuleItem, error) {
	body, _, err := get(ctx, "https://raw.githubusercontent.com/luestr/ShuntRules/main/README.md", 2<<20)
	if err != nil {
		return nil, err
	}
	matches := shuntLinkRE.FindAllStringSubmatch(string(body), -1)
	seen := map[string]bool{}
	items := make([]RuleItem, 0, len(matches))
	for _, m := range matches {
		platform, file := m[1], strings.TrimSpace(m[2])
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file)), ".")
		if ext == "lsr" {
			ext = "list"
		}
		id := "shunt_rules:" + platform + ":" + name
		if seen[id] {
			continue
		}
		seen[id] = true
		items = append(items, RuleItem{ExternalID: id, SourceKey: "shunt_rules", Name: name, Category: CategoryFor(name), Platform: platform, Format: ext, URL: "https://rule.kelee.one/" + platform + "/" + file})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Platform == items[j].Platform {
			return items[i].Name < items[j].Name
		}
		return items[i].Platform < items[j].Platform
	})
	return items, nil
}

type ghContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

func discoverIOSRuleURL(ctx context.Context, rec models.RuleCatalog) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/blackmatrix7/ios_rule_script/contents/rule/%s/%s?ref=master", rec.Platform, url.PathEscape(rec.Name))
	body, _, err := get(ctx, apiURL, 2<<20)
	if err != nil {
		return "", err
	}
	var entries []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return "", err
	}
	wantExt := ".list"
	if strings.EqualFold(rec.Platform, "Clash") {
		wantExt = ".yaml"
	}
	type candidate struct{ name, raw string }
	var candidates []candidate
	for _, entry := range entries {
		if entry.Type != "file" || entry.DownloadURL == "" || !strings.HasSuffix(strings.ToLower(entry.Name), wantExt) {
			continue
		}
		candidates = append(candidates, candidate{name: entry.Name, raw: entry.DownloadURL})
	}
	if len(candidates) == 0 {
		return "", errors.New("rule payload file not found")
	}
	sort.Slice(candidates, func(i, j int) bool {
		exactI := strings.EqualFold(candidates[i].name, rec.Name+wantExt)
		exactJ := strings.EqualFold(candidates[j].name, rec.Name+wantExt)
		if exactI != exactJ {
			return exactI
		}
		if len(candidates[i].name) != len(candidates[j].name) {
			return len(candidates[i].name) < len(candidates[j].name)
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates[0].raw, nil
}

func syncIOSRuleScript(ctx context.Context) ([]RuleItem, error) {
	items := []RuleItem{}
	for _, platform := range []string{"Clash", "Surge", "Loon"} {
		apiURL := fmt.Sprintf("https://api.github.com/repos/blackmatrix7/ios_rule_script/contents/rule/%s?ref=master", platform)
		body, _, err := get(ctx, apiURL, 8<<20)
		if err != nil {
			return nil, fmt.Errorf("%s catalog: %w", platform, err)
		}
		var entries []ghContent
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Type != "dir" || e.Name == "" {
				continue
			}
			// These entries are category containers whose children hold the real
			// rule payloads. Treating them as leaf rules creates permanent 404s.
			if e.Name == "Cloud" || e.Name == "Assassin'sCreed" {
				continue
			}
			ext := "list"
			if platform == "Clash" {
				ext = "yaml"
			}
			filename := e.Name + "." + ext
			raw := fmt.Sprintf("https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/%s/%s/%s", platform, url.PathEscape(e.Name), url.PathEscape(filename))
			id := "ios_rule_script:" + platform + ":" + e.Name
			items = append(items, RuleItem{ExternalID: id, SourceKey: "ios_rule_script", Name: e.Name, Category: CategoryFor(e.Name), Platform: platform, Format: ext, URL: raw, RemoteRevision: e.SHA})
		}
	}
	return items, nil
}

func CategoryFor(name string) string {
	n := strings.ToLower(name)
	groups := []struct {
		cat  string
		keys []string
	}{
		{"AI", []string{"openai", "chatgpt", "gemini", "claude", "anthropic", "copilot", "perplexity", "deepseek", "cursor", "grok", "mistral", "aigc", "apple_intelligence"}},
		{"广告/隐私", []string{"advert", "adguard", "adrules", "adblock", "banad", "banprogram", "baneasy", "reject", "privacy", "tracking", "phishing", "hijacking"}},
		{"流媒体", []string{"netflix", "youtube", "disney", "spotify", "primevideo", "hbo", "twitch", "bilibili", "biliintl", "tiktok", "stream", "abema", "bahamut", "hulu", "emby", "neteasemusic", "media", "iqiyi", "youku", "tvb"}},
		{"社交", []string{"telegram", "twitter", "facebook", "instagram", "whatsapp", "reddit", "discord", "line", "signal", "slack", "pixiv"}},
		{"Apple", []string{"apple", "icloud", "testflight"}},
		{"Google", []string{"google", "gstatic", "gmail", "gcp"}},
		{"Microsoft", []string{"microsoft", "onedrive", "office365", "xbox", "bing", "azure", "msft"}},
		{"游戏", []string{"steam", "epic", "ea", "origin", "playstation", "nintendo", "blizzard", "riot", "game", "ubisoft", "sony"}},
		{"开发者", []string{"github", "gitlab", "docker", "npm", "pypi", "vercel", "cloudflare", "aws", "heroku", "jetbrains", "oracle", "my_git"}},
		{"国内直连", []string{"china", "cncidr", "domestic", "direct", "lan", "lancidr", "private", "local", "wechat", "weibo", "baidu", "alibaba", "tencent", "jd", "taobao", "alipay", "115", "12306", "douyin", "zhihu", "meituan", "didi"}},
		{"代理/全球", []string{"proxy", "global", "gfw", "greatfire", "tld-not-cn", "cdn", "download", "speedtest", "unban"}},
	}
	if n == "ai" || n == "cn" {
		if n == "ai" {
			return "AI"
		}
		return "国内直连"
	}
	for _, g := range groups {
		for _, k := range g.keys {
			if strings.Contains(n, k) {
				return g.cat
			}
		}
	}
	return "其他"
}

func ListCatalog(source, platform, category, keyword string, page, pageSize int) ([]models.RuleCatalog, int64, error) {
	q := models.DB.Model(&models.RuleCatalog{})
	if source != "" {
		q = q.Where("source_key = ?", source)
	}
	if platform != "" {
		q = q.Where("platform = ?", platform)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 48
	}
	var all []models.RuleCatalog
	if err := q.Order("category asc, name asc, source_key asc").Find(&all).Error; err != nil {
		return nil, 0, err
	}
	backfillCandidateContentHashes(all)
	deduped := deduplicateCatalog(all)
	total := int64(len(deduped))
	start := (page - 1) * pageSize
	if start >= len(deduped) {
		return []models.RuleCatalog{}, total, nil
	}
	end := start + pageSize
	if end > len(deduped) {
		end = len(deduped)
	}
	return deduped[start:end], total, nil
}

func catalogPlatformRank(platform string) int {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "clash":
		return 0
	case "surge":
		return 1
	case "loon":
		return 2
	default:
		return 3
	}
}

func catalogSourceRank(sourceKey string) int {
	switch sourceKey {
	case "ios_rule_script":
		return 0
	case "shunt_rules":
		return 1
	case "sukka_ruleset":
		return 2
	case "metacubex_rules":
		return 3
	case "loyalsoldier_rules":
		return 4
	case "acl4ssr_rules":
		return 5
	case "cats_adrules":
		return 6
	default:
		return 7
	}
}

func catalogPreferred(a, b models.RuleCatalog) bool {
	pa, pb := catalogPlatformRank(a.Platform), catalogPlatformRank(b.Platform)
	if pa != pb {
		return pa < pb
	}
	sa, sb := catalogSourceRank(a.SourceKey), catalogSourceRank(b.SourceKey)
	if sa != sb {
		return sa < sb
	}
	return a.ID < b.ID
}

// backfillCandidateContentHashes computes and persists NormalizedContentHash
// lazily for already-cached records that share (RuleCount, MetadataJSON) with
// at least one other record, avoiding full-disk scans on untouched singletons.
func backfillCandidateContentHashes(records []models.RuleCatalog) {
	type bucketKey struct {
		count int
		meta  string
	}
	counts := make(map[bucketKey]int, len(records))
	for i := range records {
		if records[i].RuleCount <= 0 {
			continue
		}
		counts[bucketKey{count: records[i].RuleCount, meta: records[i].MetadataJSON}]++
	}
	type hashUpdate struct {
		externalID string
		hash       string
	}
	var updates []hashUpdate
	for i := range records {
		rec := &records[i]
		if rec.RuleCount <= 0 || rec.ContentHash != "" {
			continue
		}
		if counts[bucketKey{count: rec.RuleCount, meta: rec.MetadataJSON}] < 2 {
			continue
		}
		local := rec.LocalPath
		if local == "" {
			local = cachePath(rec.SourceKey, rec.Platform, rec.Name, rec.Format)
		}
		data, err := os.ReadFile(local)
		if err != nil || len(data) == 0 {
			continue
		}
		rules, _, err := ParseRulesWithBehavior(data, rec.Format, rec.Behavior)
		if err != nil || len(rules) == 0 {
			continue
		}
		h := NormalizedContentHash(rules)
		if h == "" {
			continue
		}
		rec.ContentHash = h
		updates = append(updates, hashUpdate{externalID: rec.ExternalID, hash: h})
	}
	if len(updates) == 0 || models.DB == nil {
		return
	}
	_ = models.DB.Transaction(func(tx *gorm.DB) error {
		for _, u := range updates {
			if err := tx.Model(&models.RuleCatalog{}).Where("external_id = ?", u.externalID).Update("content_hash", u.hash).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// deduplicateCatalog collapses entries whose normalized rule count AND
// normalized rule content hash are identical. Uncached entries (RuleCount == 0
// or empty ContentHash) are never collapsed.
func deduplicateCatalog(records []models.RuleCatalog) []models.RuleCatalog {
	out := make([]models.RuleCatalog, 0, len(records))
	seen := make(map[string]int, len(records))
	for _, rec := range records {
		if rec.RuleCount <= 0 || rec.ContentHash == "" {
			out = append(out, rec)
			continue
		}
		key := fmt.Sprintf("%d:%s", rec.RuleCount, rec.ContentHash)
		if idx, exists := seen[key]; exists {
			prevCount := out[idx].DuplicateCount
			if catalogPreferred(rec, out[idx]) {
				out[idx] = rec
			}
			out[idx].DuplicateCount = prevCount + 1
			continue
		}
		seen[key] = len(out)
		out = append(out, rec)
	}
	return out
}

func Sources() ([]SourceStatus, error) {
	if err := EnsureSources(); err != nil {
		return nil, err
	}
	var srcs []models.RuleSource
	if err := models.DB.Order("id asc").Find(&srcs).Error; err != nil {
		return nil, err
	}
	out := make([]SourceStatus, 0, len(srcs))
	for _, s := range srcs {
		var count, cached int64
		models.DB.Model(&models.RuleCatalog{}).Where("source_key = ?", s.Key).Count(&count)
		models.DB.Model(&models.RuleCatalog{}).Where("source_key = ? AND checksum <> ''", s.Key).Count(&cached)
		out = append(out, SourceStatus{Key: s.Key, Name: s.Name, Kind: s.Type, Repo: s.Repo, Branch: s.Branch, PathPrefix: s.PathPrefix, Platform: s.Platform, Behavior: s.Behavior, Custom: s.Custom, Enabled: s.Enabled, Status: s.LastSyncStatus, LastSyncAt: s.LastSyncAt, Error: s.LastSyncError, Count: count, CachedCount: cached})
	}
	return out, nil
}

func WarmSourceCache(ctx context.Context, sourceKey string) {
	if _, loaded := warmRunning.LoadOrStore(sourceKey, true); loaded {
		return
	}
	defer warmRunning.Delete(sourceKey)
	var records []models.RuleCatalog
	if err := models.DB.Where("source_key = ?", sourceKey).Order("id asc").Find(&records).Error; err != nil {
		return
	}

	// Warm up to 80 rules per sync pass sequentially at low priority so large
	// upstream catalogs (e.g. MetaCubeX 590+ files) never saturate VPS bandwidth;
	// remaining rules are cached immediately on demand when previewed or imported.
	warmed := 0
	for _, rec := range records {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if rec.Checksum != "" && rec.RemoteRevision != "" && rec.CacheRevision == rec.RemoteRevision {
			continue
		}
		_, _, _, _ = refreshCatalogRecord(ctx, rec, true)
		warmed++
		if warmed >= 80 {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
}

func refreshCatalogRecord(ctx context.Context, rec models.RuleCatalog, checkRemote bool) ([]byte, []NormalizedRule, []string, error) {
	local := rec.LocalPath
	if local == "" {
		local = cachePath(rec.SourceKey, rec.Platform, rec.Name, rec.Format)
	}
	cached, cachedErr := os.ReadFile(local)
	cacheOK := cachedErr == nil && len(cached) > 0

	if checkRemote && rec.RemoteRevision != "" && rec.CacheRevision == rec.RemoteRevision && cacheOK && rec.RuleCount > 0 {
		rules, warnings, err := ParseRulesWithBehavior(cached, rec.Format, rec.Behavior)
		return cached, rules, warnings, err
	}
	if !checkRemote && cacheOK && rec.RuleCount > 0 {
		rules, warnings, err := ParseRulesWithBehavior(cached, rec.Format, rec.Behavior)
		return cached, rules, warnings, err
	}

	body, _, fetchErr := get(ctx, rec.URL, 16<<20)
	if fetchErr != nil && rec.SourceKey == "ios_rule_script" {
		if discovered, discoverErr := discoverIOSRuleURL(ctx, rec); discoverErr == nil && discovered != "" && discovered != rec.URL {
			if retryBody, _, retryErr := get(ctx, discovered, 16<<20); retryErr == nil {
				body, fetchErr, rec.URL = retryBody, nil, discovered
				_ = models.DB.Model(&models.RuleCatalog{}).Where("external_id = ?", rec.ExternalID).Update("url", discovered).Error
			}
		}
	}
	if fetchErr != nil {
		if !cacheOK {
			return nil, nil, nil, fetchErr
		}
		rules, warnings, err := ParseRulesWithBehavior(cached, rec.Format, rec.Behavior)
		return cached, rules, warnings, err
	}
	sum := sha256.Sum256(body)
	checksum := hex.EncodeToString(sum[:])
	if cacheOK && rec.Checksum != "" && checksum == rec.Checksum {
		// The remote payload is byte-for-byte identical. Keep the existing file
		// and, critically, do not issue a no-op SQLite UPDATE.
		rules, warnings, err := ParseRulesWithBehavior(cached, rec.Format, rec.Behavior)
		if err == nil && rec.ContentHash == "" {
			if h := NormalizedContentHash(rules); h != "" {
				_ = models.DB.Model(&models.RuleCatalog{}).Where("external_id = ?", rec.ExternalID).Update("content_hash", h).Error
			}
		}
		return cached, rules, warnings, err
	}
	rules, warnings, err := ParseRulesWithBehavior(body, rec.Format, rec.Behavior)
	if err != nil {
		return nil, nil, nil, err
	}
	changed := !cacheOK || checksum != rec.Checksum
	if changed {
		if err := atomicCacheWrite(local, body); err != nil {
			return nil, nil, nil, err
		}
	}
	meta := CountTypes(rules)
	metaJSON, _ := json.Marshal(meta)
	contentHash := NormalizedContentHash(rules)
	updates := map[string]any{
		"local_path": local, "rule_count": len(rules), "checksum": checksum,
		"content_hash": contentHash, "metadata_json": string(metaJSON), "cache_revision": rec.RemoteRevision,
	}
	_ = models.DB.Model(&models.RuleCatalog{}).Where("external_id = ?", rec.ExternalID).Updates(updates).Error
	return body, rules, warnings, nil
}

func LoadItem(ctx context.Context, externalID string) (RuleItem, []NormalizedRule, error) {
	var rec models.RuleCatalog
	if err := models.DB.Where("external_id = ?", externalID).First(&rec).Error; err != nil {
		return RuleItem{}, nil, err
	}
	data, rules, warnings, err := refreshCatalogRecord(ctx, rec, false)
	if err != nil {
		return RuleItem{}, nil, err
	}
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])
	meta := CountTypes(rules)
	sample := rules
	if len(sample) > 30 {
		sample = sample[:30]
	}
	behavior := rec.Behavior
	if behavior == "" {
		behavior = "classical"
	}
	return RuleItem{ExternalID: rec.ExternalID, SourceKey: rec.SourceKey, Name: rec.Name, Category: rec.Category, Platform: rec.Platform, Format: rec.Format, Behavior: behavior, URL: rec.URL, LocalPath: cachePath(rec.SourceKey, rec.Platform, rec.Name, rec.Format), RuleCount: len(rules), UpdatedAt: rec.RemoteUpdate, RemoteRevision: rec.RemoteRevision, Checksum: checksum, Metadata: meta, Warnings: warnings, Sample: sample}, rules, nil
}

func cachePath(source, platform, name, format string) string {
	clean := regexp.MustCompile(`[^a-zA-Z0-9._-]+`).ReplaceAllString(name, "_")
	return filepath.Join(cacheRoot, source, platform, clean+"."+format)
}
func atomicCacheWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rule-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func fetchCatalogRemote(ctx context.Context, rec models.RuleCatalog) ([]byte, error) {
	body, _, err := get(ctx, rec.URL, 16<<20)
	if err == nil {
		return body, nil
	}
	if rec.SourceKey == "ios_rule_script" {
		if discovered, discoverErr := discoverIOSRuleURL(ctx, rec); discoverErr == nil && discovered != "" {
			if retry, _, retryErr := get(ctx, discovered, 16<<20); retryErr == nil {
				return retry, nil
			}
		}
	}
	return nil, err
}

func ruleIdentity(r NormalizedRule) string {
	return strings.ToUpper(r.Type) + "\x00" + strings.ToLower(strings.TrimSpace(r.Value))
}
func ruleFullKey(r NormalizedRule) string {
	return ruleIdentity(r) + "\x00" + strings.Join(r.Options, "\x00")
}

func analyzeRedundantRules(rules []NormalizedRule) (duplicates, covered int) {
	seen := map[string]bool{}
	suffixes := []string{}
	for _, r := range rules {
		key := ruleFullKey(r)
		if seen[key] {
			duplicates++
			continue
		}
		seen[key] = true
		t := strings.ToUpper(r.Type)
		value := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(r.Value), "."))
		if t == "DOMAIN" || t == "DOMAIN-SUFFIX" {
			for _, suffix := range suffixes {
				if value == suffix || strings.HasSuffix(value, "."+suffix) {
					covered++
					break
				}
			}
			if t == "DOMAIN-SUFFIX" && value != "" {
				suffixes = append(suffixes, value)
			}
		}
	}
	return
}

func prepareCatalogUpdate(ctx context.Context, externalID string) (RuleUpdatePreview, models.RuleCatalog, []byte, []byte, []NormalizedRule, error) {
	var rec models.RuleCatalog
	if err := models.DB.Where("external_id = ?", externalID).First(&rec).Error; err != nil {
		return RuleUpdatePreview{}, rec, nil, nil, nil, err
	}
	local := rec.LocalPath
	if local == "" {
		local = cachePath(rec.SourceKey, rec.Platform, rec.Name, rec.Format)
	}
	oldData, _ := os.ReadFile(local)
	oldRules := []NormalizedRule{}
	if len(oldData) > 0 {
		if parsed, _, err := ParseRulesWithBehavior(oldData, rec.Format, rec.Behavior); err == nil {
			oldRules = parsed
		}
	}
	newData, err := fetchCatalogRemote(ctx, rec)
	if err != nil {
		return RuleUpdatePreview{}, rec, oldData, nil, nil, err
	}
	newRules, warnings, err := ParseRulesWithBehavior(newData, rec.Format, rec.Behavior)
	if err != nil {
		return RuleUpdatePreview{}, rec, oldData, newData, nil, err
	}
	oldSum := sha256.Sum256(oldData)
	newSum := sha256.Sum256(newData)
	oldChecksum := ""
	if len(oldData) > 0 {
		oldChecksum = hex.EncodeToString(oldSum[:])
	}
	newChecksum := hex.EncodeToString(newSum[:])
	oldByID := map[string]NormalizedRule{}
	newByID := map[string]NormalizedRule{}
	for _, r := range oldRules {
		oldByID[ruleIdentity(r)] = r
	}
	for _, r := range newRules {
		newByID[ruleIdentity(r)] = r
	}
	added := []NormalizedRule{}
	deleted := []NormalizedRule{}
	modified := []RuleModification{}
	for id, nr := range newByID {
		if or, ok := oldByID[id]; !ok {
			added = append(added, nr)
		} else if ruleFullKey(or) != ruleFullKey(nr) {
			modified = append(modified, RuleModification{Before: or, After: nr})
		}
	}
	for id, or := range oldByID {
		if _, ok := newByID[id]; !ok {
			deleted = append(deleted, or)
		}
	}
	duplicates, covered := analyzeRedundantRules(newRules)
	sort.Slice(added, func(i, j int) bool { return ruleFullKey(added[i]) < ruleFullKey(added[j]) })
	sort.Slice(deleted, func(i, j int) bool { return ruleFullKey(deleted[i]) < ruleFullKey(deleted[j]) })
	preview := RuleUpdatePreview{ExternalID: rec.ExternalID, Name: rec.Name, Platform: rec.Platform, Format: rec.Format, OldChecksum: oldChecksum, NewChecksum: newChecksum, OldCount: len(oldRules), NewCount: len(newRules), AddedCount: len(added), DeletedCount: len(deleted), ModifiedCount: len(modified), DuplicateCount: duplicates, CoveredCount: covered, Changed: oldChecksum != newChecksum, Added: added, Deleted: deleted, Modified: modified, TypeCounts: CountTypes(newRules), Warnings: warnings}
	if len(preview.Added) > 100 {
		preview.Added = preview.Added[:100]
	}
	if len(preview.Deleted) > 100 {
		preview.Deleted = preview.Deleted[:100]
	}
	if len(preview.Modified) > 100 {
		preview.Modified = preview.Modified[:100]
	}
	return preview, rec, oldData, newData, newRules, nil
}

func PreviewCatalogUpdate(ctx context.Context, externalID string) (RuleUpdatePreview, error) {
	preview, _, _, _, _, err := prepareCatalogUpdate(ctx, externalID)
	return preview, err
}

func PreviewCatalogDomainMatches(ctx context.Context, externalID string, domains []string) ([]RuleDomainMatchDiff, error) {
	_, rec, oldData, newData, _, err := prepareCatalogUpdate(ctx, externalID)
	if err != nil {
		return nil, err
	}
	oldRules := []NormalizedRule{}
	if len(oldData) > 0 {
		oldRules, _, _ = ParseRulesWithBehavior(oldData, rec.Format, rec.Behavior)
	}
	newRules, _, err := ParseRulesWithBehavior(newData, rec.Format, rec.Behavior)
	if err != nil {
		return nil, err
	}
	out := make([]RuleDomainMatchDiff, 0, len(domains))
	for _, domain := range domains {
		before := MatchDomain(oldRules, domain)
		after := MatchDomain(newRules, domain)
		out = append(out, RuleDomainMatchDiff{Domain: domain, Before: before, After: after, Changed: before != after})
	}
	return out, nil
}

func snapshotCatalog(rec models.RuleCatalog, content []byte) error {
	if len(content) == 0 {
		return nil
	}
	snapshot := models.RuleCacheSnapshot{ExternalID: rec.ExternalID, Checksum: rec.Checksum, RuleCount: rec.RuleCount, MetadataJSON: rec.MetadataJSON, CacheRevision: rec.CacheRevision, Content: append([]byte(nil), content...)}
	if err := models.DB.Create(&snapshot).Error; err != nil {
		return err
	}
	var old []models.RuleCacheSnapshot
	_ = models.DB.Where("external_id = ?", rec.ExternalID).Order("id desc").Offset(5).Find(&old).Error
	if len(old) > 0 {
		ids := make([]uint, 0, len(old))
		for _, item := range old {
			ids = append(ids, item.ID)
		}
		_ = models.DB.Delete(&models.RuleCacheSnapshot{}, ids).Error
	}
	return nil
}

func ApplyCatalogUpdate(ctx context.Context, externalID string) (RuleUpdatePreview, error) {
	preview, rec, oldData, newData, newRules, err := prepareCatalogUpdate(ctx, externalID)
	if err != nil {
		return preview, err
	}
	if !preview.Changed {
		return preview, nil
	}
	if err := snapshotCatalog(rec, oldData); err != nil {
		return preview, err
	}
	local := rec.LocalPath
	if local == "" {
		local = cachePath(rec.SourceKey, rec.Platform, rec.Name, rec.Format)
	}
	if err := atomicCacheWrite(local, newData); err != nil {
		return preview, err
	}
	meta, _ := json.Marshal(CountTypes(newRules))
	updates := map[string]any{"local_path": local, "rule_count": len(newRules), "checksum": preview.NewChecksum, "metadata_json": string(meta), "cache_revision": rec.RemoteRevision}
	if err := models.DB.Model(&models.RuleCatalog{}).Where("external_id = ?", externalID).Updates(updates).Error; err != nil {
		return preview, err
	}
	return preview, nil
}

func RuleSnapshots(externalID string) ([]models.RuleCacheSnapshot, error) {
	var out []models.RuleCacheSnapshot
	err := models.DB.Select("id,external_id,checksum,rule_count,metadata_json,cache_revision,created_at").Where("external_id = ?", externalID).Order("id desc").Limit(20).Find(&out).Error
	return out, err
}

func RollbackCatalog(externalID string, snapshotID uint) error {
	var rec models.RuleCatalog
	if err := models.DB.Where("external_id = ?", externalID).First(&rec).Error; err != nil {
		return err
	}
	var snap models.RuleCacheSnapshot
	q := models.DB.Where("external_id = ?", externalID)
	if snapshotID > 0 {
		q = q.Where("id = ?", snapshotID)
	}
	if err := q.Order("id desc").First(&snap).Error; err != nil {
		return err
	}
	local := rec.LocalPath
	if local == "" {
		local = cachePath(rec.SourceKey, rec.Platform, rec.Name, rec.Format)
	}
	current, _ := os.ReadFile(local)
	if err := snapshotCatalog(rec, current); err != nil {
		return err
	}
	if err := atomicCacheWrite(local, snap.Content); err != nil {
		return err
	}
	return models.DB.Model(&models.RuleCatalog{}).Where("external_id = ?", externalID).Updates(map[string]any{"local_path": local, "checksum": snap.Checksum, "rule_count": snap.RuleCount, "metadata_json": snap.MetadataJSON, "cache_revision": snap.CacheRevision}).Error
}
