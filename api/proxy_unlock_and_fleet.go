package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"ppeelink/models"
	"ppeelink/node"

	"github.com/gin-gonic/gin"
)

func init() {
	node.SmartUnlockFilter = resolveSmartUnlockGroupNodes
	node.SmartUnlockProviderFilter = resolveSmartUnlockProviderFilter
}

func normalizeUnlockReportRegions(rep *serverOpsUnlockReport) {
	if rep == nil {
		return
	}
	for i := range rep.Items {
		r := strings.ToUpper(strings.TrimSpace(rep.Items[i].Region))
		if r == "" {
			continue
		}
		if rep.Items[i].Key == "netflix" {
			if r == "TITLE" || r == "WATCH" || r == "BROWSE" {
				rep.Items[i].Region = "US"
				continue
			}
			if parts := strings.Split(r, "-"); len(parts) >= 2 && len(parts[0]) == 2 {
				rep.Items[i].Region = parts[0]
				continue
			}
		}
	}
}

func groupUnlockServiceKey(groupName string) string {
	low := strings.ToLower(strings.TrimSpace(groupName))
	switch {
	case strings.Contains(low, "chatgpt") || strings.Contains(low, "openai"):
		return "chatgpt"
	case strings.Contains(low, "claude") || strings.Contains(low, "anthropic"):
		return "claude"
	case strings.Contains(low, "gemini") || strings.Contains(low, "google"):
		return "gemini"
	case low == "ai" || strings.HasPrefix(low, "ai ") || strings.HasSuffix(low, " ai"):
		return "ai"
	case strings.Contains(low, "netflix"):
		return "netflix"
	case strings.Contains(low, "disney"):
		return "disney"
	case strings.Contains(low, "tiktok"):
		return "tiktok"
	default:
		return ""
	}
}

func serverUnlocksService(rep *serverOpsUnlockReport, serviceKey string) (unlocked bool, known bool) {
	if rep == nil || len(rep.Items) == 0 {
		return false, false
	}
	byKey := make(map[string]string, len(rep.Items))
	for _, it := range rep.Items {
		byKey[strings.ToLower(it.Key)] = strings.ToLower(it.Status)
	}
	if serviceKey == "ai" {
		anyKnown := false
		for _, k := range []string{"chatgpt", "claude", "gemini"} {
			if st, ok := byKey[k]; ok && st != "" && st != "unknown" {
				anyKnown = true
				if st == "available" {
					return true, true
				}
			}
		}
		return false, anyKnown
	}
	st, ok := byKey[serviceKey]
	if !ok || st == "" || st == "unknown" {
		return false, false
	}
	if serviceKey == "netflix" {
		return st == "available" || st == "originals", true
	}
	return st == "available", true
}

// resolveSmartUnlockGroupNodes maps dual-source (Server Agent native + Clash real-tunnel) unlock results
// onto Clash subscription proxy-groups: unlocked verified nodes are auto-included, blocked verified nodes
// are auto-excluded, and unchecked/external nodes follow fallbackRegex.
func resolveSmartUnlockGroupNodes(groupName string, allProxies []string, fallbackRegex *regexp.Regexp) ([]string, bool) {
	if models.DB == nil {
		return nil, false
	}
	serviceKey := groupUnlockServiceKey(groupName)
	if serviceKey == "" {
		return nil, false
	}

	entities, _, _, _, _ := loadMergedUnlockEntities()
	if len(entities) == 0 {
		return nil, false
	}

	byHost := map[string]*serverOpsUnlockReport{}
	byName := map[string]*serverOpsUnlockReport{}
	for _, ent := range entities {
		if ent.Host != "" {
			byHost[ent.Host] = ent.Report
		}
		if nm := strings.ToLower(strings.TrimSpace(ent.MatchToken)); nm != "" {
			byName[nm] = ent.Report
		}
	}

	var dbNodes []models.Node
	_ = models.DB.Select("name, link").Find(&dbNodes).Error
	nodeHostByName := make(map[string]string, len(dbNodes))
	for _, n := range dbNodes {
		if h, _ := node.ExtractServerHost(n.Link); h != "" {
			nodeHostByName[n.Name] = strings.ToLower(strings.TrimSpace(h))
		}
	}

	out := make([]string, 0, len(allProxies))
	appliedAny := false
	for _, proxyName := range allProxies {
		var rep *serverOpsUnlockReport
		lowClean := strings.ToLower(cleanClashNodeName(proxyName))
		// Prefer exact name match first (distinguishes DMIT_USA vs DMIT_USA_v6).
		if r, ok := byName[lowClean]; ok {
			rep = r
		}
		if rep == nil {
			if h, ok := nodeHostByName[proxyName]; ok {
				rep = byHost[h]
			}
		}
		if rep == nil {
			lowP := strings.ToLower(proxyName)
			bestLen := 0
			for sName, r := range byName {
				if strings.Contains(lowP, sName) && len(sName) > bestLen {
					rep = r
					bestLen = len(sName)
				}
			}
		}
		if rep != nil {
			unlocked, known := serverUnlocksService(rep, serviceKey)
			if known {
				appliedAny = true
				if unlocked {
					out = append(out, proxyName)
				}
				continue
			}
		}
		if fallbackRegex == nil || fallbackRegex.MatchString(proxyName) {
			out = append(out, proxyName)
		}
	}
	if !appliedAny {
		return nil, false
	}
	return out, true
}

// resolveSmartUnlockProviderFilter dynamically synthesizes a Clash proxy-provider `filter` regex
// matching dual-source verified-unlocked nodes (plus any unchecked external nodes matching fallbackFilter).
func resolveSmartUnlockProviderFilter(groupName string, fallbackFilter string) (string, bool) {
	if models.DB == nil {
		return "", false
	}
	serviceKey := groupUnlockServiceKey(groupName)
	if serviceKey == "" {
		return "", false
	}
	entities, _, _, _, _ := loadMergedUnlockEntities()
	if len(entities) == 0 {
		return "", false
	}

	byHost := map[string]*serverOpsUnlockReport{}
	unlockedTokens := make([]string, 0, len(entities))
	knownCount := 0
	for _, ent := range entities {
		if ent.Host != "" {
			byHost[ent.Host] = ent.Report
		}
		if ok, known := serverUnlocksService(ent.Report, serviceKey); known {
			knownCount++
			if ok && strings.TrimSpace(ent.MatchToken) != "" {
				unlockedTokens = append(unlockedTokens, strings.TrimSpace(ent.MatchToken))
			}
		}
	}
	if knownCount == 0 || len(unlockedTokens) == 0 {
		return "", false
	}

	var fallbackRe *regexp.Regexp
	if strings.TrimSpace(fallbackFilter) != "" {
		fallbackRe, _ = regexp.Compile(fallbackFilter)
	}

	tokens := make([]string, 0, len(unlockedTokens)+4)
	seen := map[string]bool{}
	addTok := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		tokens = append(tokens, regexp.QuoteMeta(raw))
	}
	for _, nm := range unlockedTokens {
		addTok(nm)
	}

	// Preserve unchecked external nodes that match the original fallbackFilter.
	var dbNodes []models.Node
	if models.DB.Select("name, link").Find(&dbNodes).Error == nil {
		for _, n := range dbNodes {
			h, _ := node.ExtractServerHost(n.Link)
			if _, isManaged := byHost[strings.ToLower(strings.TrimSpace(h))]; isManaged {
				continue
			}
			matchedEntity := false
			lowName := strings.ToLower(n.Name)
			for _, ent := range entities {
				if sn := strings.ToLower(strings.TrimSpace(ent.MatchToken)); sn != "" && strings.Contains(lowName, sn) {
					matchedEntity = true
					break
				}
			}
			if matchedEntity {
				continue
			}
			if fallbackRe == nil || fallbackRe.MatchString(n.Name) {
				addTok(n.Name)
			}
		}
	}
	if len(tokens) == 0 {
		return "", false
	}
	return "(?i)(?:" + strings.Join(tokens, "|") + ")", true
}

// ProxyUnlockRoutingPreview 返回当前基于双源解锁矩阵（服务端原生 + 本机 Clash 实测）自动生效的订阅智能分流映射。
func ProxyUnlockRoutingPreview(c *gin.Context) {
	entities, totalServers, nativeCount, clashCount, dualCount := loadMergedUnlockEntities()

	defs := []struct {
		Key     string
		Service string
		Groups  string
	}{
		{"chatgpt", "ChatGPT", "ChatGPT / OpenAI"},
		{"claude", "Claude", "Claude / Anthropic"},
		{"gemini", "Gemini / Google", "Gemini / Google"},
		{"netflix", "Netflix", "Netflix"},
		{"disney", "Disney+", "Disney+"},
		{"tiktok", "TikTok", "TikTok"},
	}

	type groupPreview struct {
		Key           string            `json:"key"`
		Service       string            `json:"service"`
		Groups        string            `json:"groups"`
		Included      []string          `json:"included"`
		Excluded      []string          `json:"excluded"`
		SourcesByNode map[string]string `json:"sourcesByNode,omitempty"` // nodeName -> "both" | "native" | "clash"
		Filter        string            `json:"filter"`
	}
	out := make([]groupPreview, 0, len(defs))
	for _, d := range defs {
		gp := groupPreview{
			Key:           d.Key,
			Service:       d.Service,
			Groups:        d.Groups,
			Included:      []string{},
			Excluded:      []string{},
			SourcesByNode: map[string]string{},
		}
		for _, ent := range entities {
			ok, known := serverUnlocksService(ent.Report, d.Key)
			if !known {
				continue
			}
			// Determine per-service verification source for this node.
			itemSrc := ent.Source
			for _, it := range ent.Report.Items {
				if strings.EqualFold(it.Key, d.Key) && it.Source != "" {
					itemSrc = it.Source
					break
				}
			}
			gp.SourcesByNode[ent.Name] = itemSrc
			if ok {
				gp.Included = append(gp.Included, ent.Name)
			} else {
				gp.Excluded = append(gp.Excluded, ent.Name)
			}
		}
		if len(gp.Included) > 0 || len(gp.Excluded) > 0 {
			if f, applied := resolveSmartUnlockProviderFilter(d.Key, ""); applied {
				gp.Filter = f
			}
			out = append(out, gp)
		}
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"checkedServers": len(entities),
			"totalServers":   totalServers,
			"nativeCount":    nativeCount,
			"clashCount":     clashCount,
			"dualCount":      dualCount,
			"groups":         out,
		},
		"msg": "ok",
	})
}

// ProxyServerUnlockCheck 对单台或全部在线服务器发起 Server Agent 原生解锁矩阵检测并落库。
func ProxyServerUnlockCheck(c *gin.Context) {
	var req struct {
		ServerID uint `json:"serverId"`
		All      bool `json:"all"`
	}
	_ = c.ShouldBindJSON(&req)

	var targets []models.ProxyServer
	if req.All || req.ServerID == 0 {
		if err := models.DB.Where("kind = ? AND removing = ? AND enabled = ?", models.ServerKindAgent, false, true).Find(&targets).Error; err != nil {
			c.JSON(500, gin.H{"code": "50000", "msg": "读取服务器失败"})
			return
		}
	} else {
		var s models.ProxyServer
		if err := models.DB.First(&s, req.ServerID).Error; err != nil {
			c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
			return
		}
		targets = []models.ProxyServer{s}
	}

	type itemOutcome struct {
		ServerID   uint                   `json:"serverId"`
		ServerName string                 `json:"serverName"`
		OK         bool                   `json:"ok"`
		Unlock     *serverOpsUnlockReport `json:"unlock,omitempty"`
		Error      string                 `json:"error,omitempty"`
	}

	results := make([]itemOutcome, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		srv := targets[i]
		results[i] = itemOutcome{ServerID: srv.ID, ServerName: srv.Name}
		if !serverOpsOnline(&srv) {
			results[i].Error = "离线"
			continue
		}
		wg.Add(1)
		go func(idx int, s models.ProxyServer) {
			defer wg.Done()
			jobID, resCh, ok := opsDispatch(s.ID, serverOpsJob{Action: "unlock-check"}, true)
			if !ok {
				results[idx].Error = "通道繁忙"
				return
			}
			defer opsCleanup(jobID)
			select {
			case res := <-resCh:
				if !res.OK || res.Unlock == nil {
					results[idx].Error = res.Error
					if results[idx].Error == "" {
						results[idx].Error = "检测失败"
					}
					return
				}
				raw, _ := json.Marshal(res.Unlock)
				now := time.Now()
				_ = models.DB.Model(&models.ProxyServer{}).Where("id = ?", s.ID).Updates(map[string]any{
					"unlock_json":       string(raw),
					"unlock_checked_at": now,
				}).Error
				results[idx].OK = true
				results[idx].Unlock = res.Unlock
			case <-time.After(24 * time.Second):
				results[idx].Error = "超时（需 Agent v1.22+）"
			case <-c.Request.Context().Done():
				results[idx].Error = "已取消"
			}
		}(i, srv)
	}
	wg.Wait()

	okCount := 0
	for _, r := range results {
		if r.OK {
			okCount++
		}
	}
	if okCount > 0 {
		_ = SyncAllSmartTemplates(false)
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{"results": results, "okCount": okCount, "total": len(results)},
		"msg":  fmt.Sprintf("已完成 %d/%d 台服务器原生解锁检测", okCount, len(results)),
	})
}

// RunScheduledUnlockSweep 供 cron.go 每 12 小时自动执行：
// 错峰对全部在线 Server Agent 复检原生解锁能力，并在检测后自动刷新三端智能分流模板（自动摘除退化节点）。
func RunScheduledUnlockSweep() {
	if models.DB == nil {
		return
	}
	var servers []models.ProxyServer
	if err := models.DB.Where("removing = ? AND enabled = ? AND kind = ?", false, true, models.ServerKindAgent).Order("id asc").Find(&servers).Error; err != nil {
		return
	}
	okCount := 0
	for i, srv := range servers {
		if !serverOpsOnline(&srv) {
			continue
		}
		if i > 0 {
			time.Sleep(3 * time.Second)
		}
		jobID, resCh, ok := opsDispatch(srv.ID, serverOpsJob{Action: "unlock-check"}, true)
		if !ok {
			continue
		}
		select {
		case res := <-resCh:
			if res.OK && res.Unlock != nil {
				raw, _ := json.Marshal(res.Unlock)
				now := time.Now()
				_ = models.DB.Model(&models.ProxyServer{}).Where("id = ?", srv.ID).Updates(map[string]any{
					"unlock_json":       string(raw),
					"unlock_checked_at": now,
				}).Error
				okCount++
			}
		case <-time.After(28 * time.Second):
		}
		opsCleanup(jobID)
	}
	if okCount > 0 {
		_ = SyncAllSmartTemplates(false)
		log.Printf("[Cron] 定时解锁巡检完成（%d/%d 台更新），已同步三端智能分流模板", okCount, len(servers))
	}
}

// ProxyServerMigrateURL 一键向指定或全部在线服务器 Agent 下发新面板地址（含连通性预检防失联）。
func ProxyServerMigrateURL(c *gin.Context) {
	var req struct {
		PanelURL  string `json:"panelUrl"`
		ServerIDs []uint `json:"serverIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	newURL := strings.TrimRight(strings.TrimSpace(req.PanelURL), "/")
	u, err := url.Parse(newURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "请输入合法的面板地址（如 https://panel.example.com）"})
		return
	}

	var targets []models.ProxyServer
	q := models.DB.Where("kind = ? AND removing = ?", models.ServerKindAgent, false)
	if len(req.ServerIDs) > 0 {
		q = q.Where("id IN ?", req.ServerIDs)
	}
	if err := q.Find(&targets).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取服务器失败"})
		return
	}

	type migrateOutcome struct {
		ServerID   uint   `json:"serverId"`
		ServerName string `json:"serverName"`
		OK         bool   `json:"ok"`
		Message    string `json:"message"`
	}
	outcomes := make([]migrateOutcome, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		srv := targets[i]
		outcomes[i] = migrateOutcome{ServerID: srv.ID, ServerName: srv.Name}
		if !serverOpsOnline(&srv) {
			outcomes[i].Message = "离线未下发"
			continue
		}
		wg.Add(1)
		go func(idx int, s models.ProxyServer) {
			defer wg.Done()
			jobID, resCh, ok := opsDispatch(s.ID, serverOpsJob{
				Action:   "set-panel-url",
				PanelURL: newURL,
			}, true)
			if !ok {
				outcomes[idx].Message = "通道繁忙"
				return
			}
			defer opsCleanup(jobID)
			select {
			case res := <-resCh:
				outcomes[idx].OK = res.OK
				if res.OK {
					outcomes[idx].Message = res.Output
				} else {
					outcomes[idx].Message = res.Error
				}
			case <-time.After(18 * time.Second):
				outcomes[idx].Message = "超时（需 Agent v1.22+）"
			case <-c.Request.Context().Done():
				outcomes[idx].Message = "已取消"
			}
		}(i, srv)
	}
	wg.Wait()

	okCount := 0
	for _, o := range outcomes {
		if o.OK {
			okCount++
		}
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{"results": outcomes, "okCount": okCount, "total": len(outcomes)},
		"msg":  fmt.Sprintf("面板地址迁移下发完成：%d/%d 成功", okCount, len(outcomes)),
	})
}

// ProxyServerSavePolicy 批量保存并实时下发服务器 Agent 安全与隐私能力策略（WebShell / 文件 / Docker / 防火墙 / 站点统计 / 自更新）。
func ProxyServerSavePolicy(c *gin.Context) {
	var req struct {
		ServerIDs             []uint `json:"serverIds"`
		TerminalEnabled       *bool  `json:"terminalEnabled"`
		FilesEnabled          *bool  `json:"filesEnabled"`
		DockerEnabled         *bool  `json:"dockerEnabled"`
		AllowAutoUpdate       *bool  `json:"allowAutoUpdate"`
		AutoFirewall          *bool  `json:"autoFirewall"`
		WebsiteTrafficEnabled *bool  `json:"websiteTrafficEnabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}

	var targets []models.ProxyServer
	q := models.DB.Where("kind = ? AND removing = ?", models.ServerKindAgent, false)
	if len(req.ServerIDs) > 0 {
		q = q.Where("id IN ?", req.ServerIDs)
	}
	if err := q.Find(&targets).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取服务器失败"})
		return
	}

	dbUpdates := map[string]any{}
	if req.TerminalEnabled != nil {
		dbUpdates["terminal_enabled"] = *req.TerminalEnabled
	}
	if req.FilesEnabled != nil {
		dbUpdates["files_enabled"] = *req.FilesEnabled
	}
	if req.DockerEnabled != nil {
		dbUpdates["docker_enabled"] = *req.DockerEnabled
	}
	if req.AllowAutoUpdate != nil {
		dbUpdates["allow_auto_update"] = *req.AllowAutoUpdate
	}
	if req.AutoFirewall != nil {
		dbUpdates["auto_firewall"] = *req.AutoFirewall
	}
	if req.WebsiteTrafficEnabled != nil {
		dbUpdates["website_traffic_enabled"] = *req.WebsiteTrafficEnabled
	}
	if len(dbUpdates) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "未指定需要修改的策略参数"})
		return
	}

	ids := make([]uint, 0, len(targets))
	for _, s := range targets {
		ids = append(ids, s.ID)
	}
	if len(ids) > 0 {
		if err := models.DB.Model(&models.ProxyServer{}).Where("id IN ?", ids).Updates(dbUpdates).Error; err != nil {
			c.JSON(500, gin.H{"code": "50000", "msg": "保存策略失败"})
			return
		}
		if req.AutoFirewall != nil || req.WebsiteTrafficEnabled != nil {
			NotifyProxyChanged()
		}
	}

	// Re-fetch updated targets and push live set-policy to online Server Agents.
	var fresh []models.ProxyServer
	_ = models.DB.Where("id IN ?", ids).Find(&fresh).Error

	boolOrTrue := func(b *bool) bool { return b == nil || *b }
	pushed := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range fresh {
		s := fresh[i]
		if !serverOpsOnline(&s) {
			continue
		}
		wg.Add(1)
		go func(srv models.ProxyServer) {
			defer wg.Done()
			pol := &serverOpsPolicy{
				TerminalEnabled: boolOrTrue(srv.TerminalEnabled),
				FilesEnabled:    boolOrTrue(srv.FilesEnabled),
				DockerEnabled:   boolOrTrue(srv.DockerEnabled),
				AllowAutoUpdate: boolOrTrue(srv.AllowAutoUpdate),
			}
			jobID, resCh, ok := opsDispatch(srv.ID, serverOpsJob{
				Action: "set-policy",
				Policy: pol,
			}, true)
			if !ok {
				return
			}
			defer opsCleanup(jobID)
			select {
			case res := <-resCh:
				if res.OK {
					mu.Lock()
					pushed++
					mu.Unlock()
				}
			case <-time.After(10 * time.Second):
			case <-c.Request.Context().Done():
			}
		}(s)
	}
	wg.Wait()

	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{"updated": len(fresh), "pushed": pushed},
		"msg":  fmt.Sprintf("已更新 %d 台服务器策略（%d 台在线实时生效）", len(fresh), pushed),
	})
}

// ProbePushConfig 一键向指定或全部在线用户 Agent 下发面板地址迁移或隐私/Clash 运行参数。
func ProbePushConfig(c *gin.Context) {
	var req struct {
		ProbeIDs       []uint `json:"probeIds"`
		PanelURL       string `json:"panelUrl"`
		Concurrency    int    `json:"concurrency"`
		ReportPublicIP *bool  `json:"reportPublicIp"`
		AllowClash     *bool  `json:"allowClash"`
		ClashAPI       string `json:"clashApi"`
		ClashSecret    string `json:"clashSecret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	if req.PanelURL != "" {
		clean := strings.TrimRight(strings.TrimSpace(req.PanelURL), "/")
		u, err := url.Parse(clean)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			c.JSON(400, gin.H{"code": "40000", "msg": "新面板地址格式无效"})
			return
		}
		req.PanelURL = clean
	}

	var probes []models.Probe
	q := models.DB.Where("enabled = ?", true)
	if len(req.ProbeIDs) > 0 {
		q = q.Where("id IN ?", req.ProbeIDs)
	}
	if err := q.Find(&probes).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}

	dbUpdates := map[string]any{}
	if req.ReportPublicIP != nil {
		dbUpdates["report_public_ip"] = *req.ReportPublicIP
	}
	if req.AllowClash != nil {
		dbUpdates["allow_clash"] = *req.AllowClash
	}
	if req.Concurrency >= 1 && req.Concurrency <= 8 {
		dbUpdates["concurrency"] = req.Concurrency
	}
	if strings.TrimSpace(req.ClashAPI) != "" {
		dbUpdates["clash_api"] = strings.TrimSpace(req.ClashAPI)
	}
	if strings.TrimSpace(req.ClashSecret) != "" {
		dbUpdates["clash_secret"] = strings.TrimSpace(req.ClashSecret)
	}
	if len(dbUpdates) > 0 && len(probes) > 0 {
		ids := make([]uint, 0, len(probes))
		for _, p := range probes {
			ids = append(ids, p.ID)
		}
		_ = models.DB.Model(&models.Probe{}).Where("id IN ?", ids).Updates(dbUpdates).Error
	}

	patch := &userAgentConfigPatchDTO{
		PanelURL:       req.PanelURL,
		Concurrency:    req.Concurrency,
		ReportPublicIP: req.ReportPublicIP,
		AllowClash:     req.AllowClash,
		ClashAPI:       strings.TrimSpace(req.ClashAPI),
		ClashSecret:    strings.TrimSpace(req.ClashSecret),
	}

	type pushOutcome struct {
		ProbeID   uint   `json:"probeId"`
		ProbeName string `json:"probeName"`
		OK        bool   `json:"ok"`
		Message   string `json:"message"`
	}
	outcomes := make([]pushOutcome, len(probes))
	var wg sync.WaitGroup
	for i := range probes {
		p := probes[i]
		outcomes[i] = pushOutcome{ProbeID: p.ID, ProbeName: p.Name}
		if !isProbeRecentlyOnline(&p) {
			outcomes[i].Message = "离线（已保存配置）"
			continue
		}
		wg.Add(1)
		go func(idx int, pr models.Probe) {
			defer wg.Done()
			jobID, doneCh, ok := probeHub.dispatchJob(pr.ID, queuedProbeJob{
				Action: "set-config",
				Config: patch,
			}, true)
			if !ok {
				outcomes[idx].Message = "设备繁忙"
				return
			}
			defer probeHub.cleanupDone(jobID)
			select {
			case out := <-doneCh:
				outcomes[idx].OK = out.OK
				if out.OK {
					outcomes[idx].Message = "已实时下发生效"
				} else {
					outcomes[idx].Message = out.Error
				}
			case <-time.After(15 * time.Second):
				outcomes[idx].Message = "超时（需用户 Agent v1.8+）"
			case <-c.Request.Context().Done():
				outcomes[idx].Message = "已取消"
			}
		}(i, p)
	}
	wg.Wait()

	okCount := 0
	for _, o := range outcomes {
		if o.OK {
			okCount++
		}
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{"results": outcomes, "okCount": okCount, "total": len(outcomes)},
		"msg":  fmt.Sprintf("已向 %d/%d 台在线用户 Agent 下发配置", okCount, len(outcomes)),
	})
}
