package api

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"ppeelink/models"
	"ppeelink/node"

	"gorm.io/gorm/clause"
)

var unlockServiceMeta = []struct {
	Key  string
	Name string
}{
	{"chatgpt", "ChatGPT"},
	{"claude", "Claude"},
	{"gemini", "Gemini"},
	{"netflix", "Netflix"},
	{"disney", "Disney+"},
	{"youtube", "YouTube"},
	{"tiktok", "TikTok"},
	{"spotify", "Spotify"},
}

func unlockServiceDisplayName(key string) string {
	low := strings.ToLower(strings.TrimSpace(key))
	for _, m := range unlockServiceMeta {
		if m.Key == low {
			return m.Name
		}
	}
	return key
}

// cleanClashNodeName 去除 Clash 节点名前导国旗 Emoji、符号与空白，便于与 ProxyServer 精确匹配。
func cleanClashNodeName(raw string) string {
	s := strings.TrimSpace(raw)
	for len(s) > 0 {
		runes := []rune(s)
		r := runes[0]
		// Strip regional indicator flags (U+1F1E6..U+1F1FF), symbols, variation selectors, ZWJ, spaces.
		if unicode.IsSpace(r) ||
			(r >= 0x1F1E6 && r <= 0x1F1FF) ||
			(r >= 0x1F300 && r <= 0x1FAFF) ||
			r == 0xFE0F || r == 0x200D || r == '|' || r == '·' || r == '-' {
			s = strings.TrimSpace(string(runes[1:]))
			continue
		}
		break
	}
	if s == "" {
		return strings.TrimSpace(raw)
	}
	return s
}

func parseClashStatusField(raw string) (status string, region string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown", ""
	}
	parts := strings.SplitN(raw, ":", 2)
	st := strings.ToLower(strings.TrimSpace(parts[0]))
	if len(parts) == 2 {
		region = strings.ToUpper(strings.TrimSpace(parts[1]))
	}
	switch st {
	case "available", "originals", "blocked", "unknown":
		return st, region
	default:
		return "unknown", ""
	}
}

func clashDTOToUnlockItems(item clashNodeTestResultDTO) []serverOpsUnlockItem {
	pairs := []struct {
		key string
		raw string
	}{
		{"chatgpt", item.ChatGPT},
		{"claude", item.Claude},
		{"gemini", item.Gemini},
		{"netflix", item.Netflix},
		{"disney", item.Disney},
		{"youtube", item.YouTube},
	}
	out := make([]serverOpsUnlockItem, 0, len(pairs))
	for _, p := range pairs {
		st, reg := parseClashStatusField(p.raw)
		out = append(out, serverOpsUnlockItem{
			Key:    p.key,
			Name:   unlockServiceDisplayName(p.key),
			Status: st,
			Region: reg,
		})
	}
	rep := serverOpsUnlockReport{Items: out}
	normalizeUnlockReportRegions(&rep)
	return rep.Items
}

func isKnownUnlockStatus(st string) bool {
	s := strings.ToLower(strings.TrimSpace(st))
	return s == "available" || s == "originals" || s == "blocked"
}

func isUnlockedStatus(key, st string) bool {
	s := strings.ToLower(strings.TrimSpace(st))
	if strings.ToLower(strings.TrimSpace(key)) == "netflix" {
		return s == "available" || s == "originals"
	}
	return s == "available"
}

// mergeUnlockReports 合并「服务端 Agent 原生检测」与「本机 Clash 实测」两套解锁报告：
// - 两者均有有效结论：组合输出（保留各自状态），冲突时以 Clash 实测为准（代表真实用户链路）；
// - 仅有一方有有效结论：直接使用该方结果。
func mergeUnlockReports(nativeRep, clashRep *serverOpsUnlockReport) *serverOpsUnlockReport {
	if nativeRep == nil && clashRep == nil {
		return nil
	}
	if nativeRep != nil {
		cp := *nativeRep
		cp.Items = append([]serverOpsUnlockItem(nil), nativeRep.Items...)
		normalizeUnlockReportRegions(&cp)
		nativeRep = &cp
	}
	if clashRep != nil {
		cp := *clashRep
		cp.Items = append([]serverOpsUnlockItem(nil), clashRep.Items...)
		normalizeUnlockReportRegions(&cp)
		clashRep = &cp
	}

	nativeByKey := map[string]serverOpsUnlockItem{}
	if nativeRep != nil {
		for _, it := range nativeRep.Items {
			nativeByKey[strings.ToLower(strings.TrimSpace(it.Key))] = it
		}
	}
	clashByKey := map[string]serverOpsUnlockItem{}
	if clashRep != nil {
		for _, it := range clashRep.Items {
			clashByKey[strings.ToLower(strings.TrimSpace(it.Key))] = it
		}
	}

	out := &serverOpsUnlockReport{
		Items: make([]serverOpsUnlockItem, 0, len(unlockServiceMeta)),
	}
	hasNative := false
	hasClash := false

	if nativeRep != nil {
		out.IP = nativeRep.IP
		out.Country = nativeRep.Country
		out.ISP = nativeRep.ISP
		out.ASN = nativeRep.ASN
		out.Hosting = nativeRep.Hosting
		out.CheckedAt = nativeRep.CheckedAt
		hasNative = len(nativeRep.Items) > 0
	}
	if clashRep != nil {
		if out.IP == "" {
			out.IP = clashRep.IP
		}
		if out.Country == "" {
			out.Country = clashRep.Country
		}
		if out.CheckedAt == "" {
			out.CheckedAt = clashRep.CheckedAt
		}
		out.ClashCheckedAt = clashRep.ClashCheckedAt
		if out.ClashCheckedAt == "" {
			out.ClashCheckedAt = clashRep.CheckedAt
		}
		out.ClashNode = clashRep.ClashNode
		out.ClashTTFBMs = clashRep.ClashTTFBMs
		out.ClashMbps = clashRep.ClashMbps
		hasClash = len(clashRep.Items) > 0
	}
	if hasNative {
		out.Sources = append(out.Sources, "native")
	}
	if hasClash {
		out.Sources = append(out.Sources, "clash")
	}

	seen := map[string]bool{}
	keys := make([]string, 0, len(unlockServiceMeta)+2)
	for _, m := range unlockServiceMeta {
		keys = append(keys, m.Key)
		seen[m.Key] = true
	}
	for k := range nativeByKey {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	for k := range clashByKey {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}

	for _, key := range keys {
		nItem, hasN := nativeByKey[key]
		cItem, hasC := clashByKey[key]
		if !hasN && !hasC {
			continue
		}
		nKnown := hasN && isKnownUnlockStatus(nItem.Status)
		cKnown := hasC && isKnownUnlockStatus(cItem.Status)

		name := unlockServiceDisplayName(key)
		if hasN && nItem.Name != "" {
			name = nItem.Name
		} else if hasC && cItem.Name != "" {
			name = cItem.Name
		}

		merged := serverOpsUnlockItem{
			Key:  key,
			Name: name,
		}
		switch {
		case nKnown && cKnown:
			// 双源均已验证：组合输出，冲突时以 Clash 实测优先。
			merged.Source = "both"
			merged.NativeStatus = strings.ToLower(strings.TrimSpace(nItem.Status))
			merged.NativeRegion = strings.ToUpper(strings.TrimSpace(nItem.Region))
			merged.ClashStatus = strings.ToLower(strings.TrimSpace(cItem.Status))
			merged.ClashRegion = strings.ToUpper(strings.TrimSpace(cItem.Region))
			merged.Status = merged.ClashStatus
			merged.Region = merged.ClashRegion
			if merged.Region == "" && isUnlockedStatus(key, merged.Status) && isUnlockedStatus(key, merged.NativeStatus) {
				merged.Region = merged.NativeRegion
			}
			if isUnlockedStatus(key, merged.NativeStatus) == isUnlockedStatus(key, merged.ClashStatus) {
				merged.Consensus = "agree"
			} else {
				merged.Consensus = "conflict"
			}
		case cKnown:
			merged.Source = "clash"
			merged.ClashStatus = strings.ToLower(strings.TrimSpace(cItem.Status))
			merged.ClashRegion = strings.ToUpper(strings.TrimSpace(cItem.Region))
			merged.Status = merged.ClashStatus
			merged.Region = merged.ClashRegion
		case nKnown:
			merged.Source = "native"
			merged.NativeStatus = strings.ToLower(strings.TrimSpace(nItem.Status))
			merged.NativeRegion = strings.ToUpper(strings.TrimSpace(nItem.Region))
			merged.Status = merged.NativeStatus
			merged.Region = merged.NativeRegion
		default:
			merged.Status = "unknown"
			if hasC && !hasN {
				merged.Source = "clash"
			} else if hasN && !hasC {
				merged.Source = "native"
			} else {
				merged.Source = "both"
			}
		}
		out.Items = append(out.Items, merged)
	}
	if len(out.Items) == 0 {
		return nil
	}
	return out
}

func clashSnapshotToReport(snap *models.ClashUnlockSnapshot) *serverOpsUnlockReport {
	if snap == nil || snap.ItemsJSON == "" {
		return nil
	}
	var items []serverOpsUnlockItem
	if json.Unmarshal([]byte(snap.ItemsJSON), &items) != nil || len(items) == 0 {
		return nil
	}
	ts := snap.CheckedAt.UTC().Format(time.RFC3339)
	return &serverOpsUnlockReport{
		IP:             snap.EgressIP,
		Country:        snap.Country,
		CheckedAt:      ts,
		ClashCheckedAt: ts,
		ClashNode:      snap.NodeName,
		ClashTTFBMs:    snap.TTFBMs,
		ClashMbps:      snap.DownloadMbps,
		Sources:        []string{"clash"},
		Items:          items,
	}
}

func matchClashNodeToServerID(rawName, cleanName string, servers []models.ProxyServer, nodeHostByName map[string]string) *uint {
	lowClean := strings.ToLower(strings.TrimSpace(cleanName))
	lowRaw := strings.ToLower(strings.TrimSpace(rawName))

	// 1. Exact case-insensitive name match (prevents DMIT_USA_v6 from stealing DMIT_USA's exact match).
	for i := range servers {
		sName := strings.ToLower(strings.TrimSpace(servers[i].Name))
		if sName != "" && (lowClean == sName || lowRaw == sName) {
			id := servers[i].ID
			return &id
		}
	}

	// 2. Match via panel Node -> server PublicHost, only if node name doesn't carry a distinct suffix like _v6
	// when an exact IPv4 node already exists, or when unambiguous.
	if host, ok := nodeHostByName[lowRaw]; ok && host != "" {
		for i := range servers {
			if strings.EqualFold(strings.TrimSpace(servers[i].PublicHost), host) {
				id := servers[i].ID
				return &id
			}
		}
	}
	if host, ok := nodeHostByName[lowClean]; ok && host != "" {
		for i := range servers {
			if strings.EqualFold(strings.TrimSpace(servers[i].PublicHost), host) {
				id := servers[i].ID
				return &id
			}
		}
	}
	return nil
}

// saveClashBatchUnlockSnapshots 将本机 Clash 批量实测结果持久化至 ClashUnlockSnapshot，
// 自动关联自建服务器并刷新 clash-smart.yaml 默认智能分流模板。
func saveClashBatchUnlockSnapshots(probeName string, items []clashNodeTestResultDTO) int {
	if models.DB == nil || len(items) == 0 {
		return 0
	}
	if !models.DB.Migrator().HasTable(&models.ClashUnlockSnapshot{}) {
		return 0
	}

	var servers []models.ProxyServer
	_ = models.DB.Where("removing = ?", false).Find(&servers).Error

	var dbNodes []models.Node
	_ = models.DB.Select("name, link").Find(&dbNodes).Error
	nodeHostByName := make(map[string]string, len(dbNodes)*2)
	for _, n := range dbNodes {
		if h, _ := node.ExtractServerHost(n.Link); h != "" {
			lh := strings.ToLower(strings.TrimSpace(h))
			nodeHostByName[strings.ToLower(strings.TrimSpace(n.Name))] = lh
			nodeHostByName[strings.ToLower(cleanClashNodeName(n.Name))] = lh
		}
	}

	now := time.Now()
	saved := 0
	for _, it := range items {
		rawName := strings.TrimSpace(it.Name)
		if rawName == "" || it.Error != "" {
			continue
		}
		// Guard: if Clash leaked to China mainland direct exit (CN), never pollute the unlock matrix.
		if strings.EqualFold(strings.TrimSpace(it.Country), "CN") {
			continue
		}
		unlockItems := clashDTOToUnlockItems(it)
		knownAny := false
		for _, ui := range unlockItems {
			if isKnownUnlockStatus(ui.Status) {
				knownAny = true
				break
			}
		}
		if !knownAny {
			continue
		}

		clean := cleanClashNodeName(rawName)
		// If clean name has "_v6" suffix and another server without "_v6" exists, keep _v6 as standalone
		// so it doesn't overwrite the primary IPv4 server snapshot unless the server itself is named *_v6.
		var srvID *uint
		if !strings.HasSuffix(strings.ToLower(clean), "_v6") {
			srvID = matchClashNodeToServerID(rawName, clean, servers, nodeHostByName)
		} else {
			for i := range servers {
				if strings.EqualFold(strings.TrimSpace(servers[i].Name), clean) {
					id := servers[i].ID
					srvID = &id
					break
				}
			}
		}

		rawJSON, err := json.Marshal(unlockItems)
		if err != nil {
			continue
		}
		row := models.ClashUnlockSnapshot{
			NodeName:     rawName,
			CleanName:    clean,
			ServerID:     srvID,
			ProbeName:    probeName,
			EgressIP:     strings.TrimSpace(it.EgressIP),
			Country:      strings.ToUpper(strings.TrimSpace(it.Country)),
			TTFBMs:       it.TTFBMs,
			DownloadMbps: it.DownloadMbps,
			ItemsJSON:    string(rawJSON),
			CheckedAt:    now,
		}
		if err := models.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "node_name"}},
			DoUpdates: clause.AssignmentColumns([]string{"clean_name", "server_id", "probe_name", "egress_ip", "country", "ttfb_ms", "download_mbps", "items_json", "checked_at"}),
		}).Create(&row).Error; err == nil {
			saved++
		}
	}
	if saved > 0 {
		_ = SyncSmartClashTemplateFile(false)
	}
	return saved
}

func loadClashSnapshotsByServerID() (map[uint]*models.ClashUnlockSnapshot, []models.ClashUnlockSnapshot) {
	byServer := map[uint]*models.ClashUnlockSnapshot{}
	if models.DB == nil || !models.DB.Migrator().HasTable(&models.ClashUnlockSnapshot{}) {
		return byServer, nil
	}
	var all []models.ClashUnlockSnapshot
	if models.DB.Order("checked_at desc, id desc").Find(&all).Error != nil {
		return byServer, nil
	}
	for i := range all {
		snap := &all[i]
		if snap.ServerID != nil && *snap.ServerID > 0 {
			if _, exists := byServer[*snap.ServerID]; !exists {
				byServer[*snap.ServerID] = snap
			}
		}
	}
	return byServer, all
}

// buildMergedServerUnlockReport 返回指定服务器的双源融合解锁报告（原生 + Clash）。
func buildMergedServerUnlockReport(s *models.ProxyServer, clashByServer map[uint]*models.ClashUnlockSnapshot) *serverOpsUnlockReport {
	if s == nil {
		return nil
	}
	var nativeRep *serverOpsUnlockReport
	if s.UnlockJSON != "" {
		var rep serverOpsUnlockReport
		if json.Unmarshal([]byte(s.UnlockJSON), &rep) == nil && len(rep.Items) > 0 {
			normalizeUnlockReportRegions(&rep)
			nativeRep = &rep
		}
	}
	var clashRep *serverOpsUnlockReport
	if clashByServer != nil {
		if snap := clashByServer[s.ID]; snap != nil {
			clashRep = clashSnapshotToReport(snap)
		}
	}
	return mergeUnlockReports(nativeRep, clashRep)
}

// mergedUnlockEntity 代表一个已验证的节点/服务器（可能来自原生、Clash 或双源融合）。
type mergedUnlockEntity struct {
	Name       string // 用于策略组与预览展示的名称（自建服为 server.Name，独立 Clash 节点为 CleanName）
	MatchToken string // 用于匹配 Clash 节点名的关键词
	Host       string // 若为自建服则为 PublicHost（小写）
	Source     string // "both" | "native" | "clash"
	Report     *serverOpsUnlockReport
}

// loadMergedUnlockEntities 汇总全部「自建服务器（双源融合）」与「独立 Clash 实测节点」，
// 作为订阅智能分流、预览接口与 clash-smart.yaml 模板联动的单一事实源。
func loadMergedUnlockEntities() (entities []mergedUnlockEntity, totalServers int, nativeCount int, clashCount int, dualCount int) {
	if models.DB == nil {
		return nil, 0, 0, 0, 0
	}
	var servers []models.ProxyServer
	_ = models.DB.Where("removing = ?", false).Order("id asc").Find(&servers).Error
	for _, s := range servers {
		if s.Kind == models.ServerKindAgent {
			totalServers++
		}
	}

	clashByServer, allClash := loadClashSnapshotsByServerID()
	usedClashNode := map[string]bool{}

	for i := range servers {
		s := &servers[i]
		var nativeRep *serverOpsUnlockReport
		if s.UnlockJSON != "" {
			var rep serverOpsUnlockReport
			if json.Unmarshal([]byte(s.UnlockJSON), &rep) == nil && len(rep.Items) > 0 {
				normalizeUnlockReportRegions(&rep)
				nativeRep = &rep
			}
		}
		var clashRep *serverOpsUnlockReport
		if snap := clashByServer[s.ID]; snap != nil {
			clashRep = clashSnapshotToReport(snap)
			usedClashNode[snap.NodeName] = true
		}
		merged := mergeUnlockReports(nativeRep, clashRep)
		if merged == nil {
			continue
		}
		src := "native"
		switch {
		case nativeRep != nil && clashRep != nil:
			src = "both"
			nativeCount++
			clashCount++
			dualCount++
		case clashRep != nil:
			src = "clash"
			clashCount++
		default:
			nativeCount++
		}
		entities = append(entities, mergedUnlockEntity{
			Name:       strings.TrimSpace(s.Name),
			MatchToken: strings.TrimSpace(s.Name),
			Host:       strings.ToLower(strings.TrimSpace(s.PublicHost)),
			Source:     src,
			Report:     merged,
		})
	}

	// Include standalone Clash-verified nodes (third-party, relays, IPv6 variants, etc.)
	for i := range allClash {
		snap := &allClash[i]
		if usedClashNode[snap.NodeName] {
			continue
		}
		if snap.ServerID != nil && *snap.ServerID > 0 {
			// Another snapshot already matched this server ID.
			continue
		}
		rep := clashSnapshotToReport(snap)
		if rep == nil {
			continue
		}
		clashCount++
		displayName := strings.TrimSpace(snap.CleanName)
		if displayName == "" {
			displayName = strings.TrimSpace(snap.NodeName)
		}
		entities = append(entities, mergedUnlockEntity{
			Name:       displayName,
			MatchToken: displayName,
			Source:     "clash",
			Report:     rep,
		})
	}
	return entities, totalServers, nativeCount, clashCount, dualCount
}
