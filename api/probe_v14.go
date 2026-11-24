package api

import (
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/tracecore"

	"github.com/gin-gonic/gin"
)

type userUnlockEgress struct {
	IP          string `json:"ip"`
	Country     string `json:"country"`
	Loc         string `json:"loc"`
	City        string `json:"city,omitempty"`
	Colo        string `json:"colo,omitempty"`
	ISP         string `json:"isp,omitempty"`
	ASN         string `json:"asn,omitempty"`
	Hosting     bool   `json:"hosting"`
	RTT         int    `json:"rtt"`
	DirectIP    string `json:"directIp,omitempty"`
	MatchedNode string `json:"matchedNode,omitempty"`
}

type userUnlockItem struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	OK        bool   `json:"ok"`
	Status    string `json:"status"`
	Region    string `json:"region,omitempty"`
	Note      string `json:"note"`
	Evidence  string `json:"evidence"`
	LatencyMs int    `json:"latencyMs"`
	CheckedAt string `json:"checkedAt"`
}

type userUnlockReport struct {
	JobID     string           `json:"jobId"`
	ProbeName string           `json:"probeName,omitempty"`
	Egress    userUnlockEgress `json:"egress"`
	Items     []userUnlockItem `json:"items"`
	Error     string           `json:"error,omitempty"`
}

type userSpeedSample struct {
	ElapsedMs int     `json:"elapsedMs"`
	Mbps      float64 `json:"mbps"`
}

type userSpeedChannel struct {
	Mode         string            `json:"mode"`
	DownloadMbps float64           `json:"downloadMbps"`
	PeakMbps     float64           `json:"peakMbps"`
	UploadMbps   float64           `json:"uploadMbps"`
	TTFBMs       int               `json:"ttfbMs"`
	JitterMs     int               `json:"jitterMs"`
	BytesDown    int64             `json:"bytesDown"`
	BytesUp      int64             `json:"bytesUp"`
	DurationMs   int               `json:"durationMs"`
	Samples      []userSpeedSample `json:"samples"`
	Error        string            `json:"error,omitempty"`
}

type userSpeedtestReport struct {
	JobID        string            `json:"jobId"`
	ProbeName    string            `json:"probeName,omitempty"`
	Mode         string            `json:"mode"`
	Proxy        *userSpeedChannel `json:"proxy,omitempty"`
	Direct       *userSpeedChannel `json:"direct,omitempty"`
	RetentionPct *float64          `json:"retentionPct,omitempty"`
	Error        string            `json:"error,omitempty"`
}

var (
	unlockResultsHub = struct {
		mu      sync.Mutex
		waiters map[string]chan userUnlockReport
	}{waiters: map[string]chan userUnlockReport{}}

	speedtestResultsHub = struct {
		mu      sync.Mutex
		waiters map[string]chan userSpeedtestReport
	}{waiters: map[string]chan userSpeedtestReport{}}
)

func pickOnlineUserProbe(probeID uint) (*models.Probe, error) {
	probes, err := models.GetProbes(true)
	if err != nil {
		return nil, err
	}
	var fallback *models.Probe
	for i := range probes {
		p := &probes[i]
		if probeID > 0 && p.ID != probeID {
			continue
		}
		if probeHub.isWaiting(p.ID) {
			return p, nil
		}
		if fallback == nil && isProbeRecentlyOnline(p) {
			fallback = p
		}
	}
	return fallback, nil
}

// ProbeUnlock 管理端：由在线用户 Agent 执行无 CORS 原生流媒体/AI 解锁检测（跟随当前代理/TUN）。
func ProbeUnlock(c *gin.Context) {
	var req struct {
		ProbeID uint `json:"probeId"`
	}
	_ = c.ShouldBindJSON(&req)

	target, err := pickOnlineUserProbe(req.ProbeID)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}
	if target == nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "没有在线的用户 Agent，请先在本机启动用户 Agent"})
		return
	}

	jobID, _, ok := probeHub.dispatchJob(target.ID, queuedProbeJob{Action: "unlock"}, true)
	if !ok {
		c.JSON(409, gin.H{"code": "40900", "msg": "用户 Agent 繁忙，请稍后再试"})
		return
	}
	defer probeHub.cleanupDone(jobID)

	resCh := make(chan userUnlockReport, 1)
	unlockResultsHub.mu.Lock()
	unlockResultsHub.waiters[jobID] = resCh
	unlockResultsHub.mu.Unlock()
	defer func() {
		unlockResultsHub.mu.Lock()
		delete(unlockResultsHub.waiters, jobID)
		unlockResultsHub.mu.Unlock()
	}()

	select {
	case res := <-resCh:
		res.ProbeName = target.Name
		if res.Egress.DirectIP != "" {
			_ = models.DB.Model(&models.Probe{}).Where("id = ?", target.ID).Update("public_ip", res.Egress.DirectIP).Error
		}
		if res.Egress.IP != "" {
			if nodes, err := models.GetNodeList(); err == nil {
				for _, n := range nodes {
					h, _ := node.ExtractServerHost(n.Link)
					if h != "" && strings.EqualFold(h, res.Egress.IP) {
						res.Egress.MatchedNode = n.Name
						break
					}
				}
			}
		}
		c.JSON(200, gin.H{"code": "00000", "data": res, "msg": "原生解锁检测完成"})
	case <-time.After(28 * time.Second):
		c.JSON(504, gin.H{"code": "50400", "msg": "解锁检测超时，请确认本机用户 Agent 已升级至 v1.4"})
	case <-c.Request.Context().Done():
		return
	}
}

// UserProbeUnlockReport 用户 Agent 回报原生解锁检测结果。
func UserProbeUnlockReport(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
		return
	}
	var req userUnlockReport
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "格式错误"})
		return
	}
	unlockResultsHub.mu.Lock()
	ch, ok := unlockResultsHub.waiters[req.JobID]
	if ok {
		delete(unlockResultsHub.waiters, req.JobID)
	}
	unlockResultsHub.mu.Unlock()
	if ok {
		select {
		case ch <- req:
		default:
		}
		close(ch)
	}
	probeHub.complete(req.JobID)
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

// ProbeSpeedtest 管理端：由在线用户 Agent 执行按需 5 秒真实带宽测速（当前代理 / 物理直连 / 对比）。
func ProbeSpeedtest(c *gin.Context) {
	var req struct {
		ProbeID uint   `json:"probeId"`
		Mode    string `json:"mode"` // proxy | direct | both
	}
	_ = c.ShouldBindJSON(&req)
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "proxy" && mode != "direct" && mode != "both" {
		mode = "proxy"
	}

	target, err := pickOnlineUserProbe(req.ProbeID)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}
	if target == nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "没有在线的用户 Agent，请先在本机启动用户 Agent"})
		return
	}

	jobID, _, ok := probeHub.dispatchJob(target.ID, queuedProbeJob{
		Action:    "speedtest",
		SpeedMode: mode,
	}, true)
	if !ok {
		c.JSON(409, gin.H{"code": "40900", "msg": "用户 Agent 繁忙，请稍后再试"})
		return
	}
	defer probeHub.cleanupDone(jobID)

	resCh := make(chan userSpeedtestReport, 1)
	speedtestResultsHub.mu.Lock()
	speedtestResultsHub.waiters[jobID] = resCh
	speedtestResultsHub.mu.Unlock()
	defer func() {
		speedtestResultsHub.mu.Lock()
		delete(speedtestResultsHub.waiters, jobID)
		speedtestResultsHub.mu.Unlock()
	}()

	select {
	case res := <-resCh:
		res.ProbeName = target.Name
		if res.Proxy != nil && res.Direct != nil && res.Direct.DownloadMbps > 0.1 {
			pct := math.Round((res.Proxy.DownloadMbps/res.Direct.DownloadMbps)*1000) / 10
			res.RetentionPct = &pct
		}
		c.JSON(200, gin.H{"code": "00000", "data": res, "msg": "本机真实带宽测速完成"})
	case <-time.After(32 * time.Second):
		c.JSON(504, gin.H{"code": "50400", "msg": "带宽测速超时，请确认本机用户 Agent 已升级至 v1.4"})
	case <-c.Request.Context().Done():
		return
	}
}

// UserProbeSpeedtestReport 用户 Agent 回报真实带宽测速结果。
func UserProbeSpeedtestReport(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
		return
	}
	var req userSpeedtestReport
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "格式错误"})
		return
	}
	speedtestResultsHub.mu.Lock()
	ch, ok := speedtestResultsHub.waiters[req.JobID]
	if ok {
		delete(speedtestResultsHub.waiters, req.JobID)
	}
	speedtestResultsHub.mu.Unlock()
	if ok {
		select {
		case ch <- req:
		default:
		}
		close(ch)
	}
	probeHub.complete(req.JobID)
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

func summarizePathQuality(hops []userTraceHop) string {
	for _, h := range hops {
		switch h.LineTag {
		case "CN2 GIA", "联通 9929", "CMIN2":
			return h.LineTag
		}
	}
	for _, h := range hops {
		switch h.LineTag {
		case "移动 CMI", "联通 4837", "IIJ", "SoftBank", "NTT":
			return h.LineTag
		}
	}
	for _, h := range hops {
		if h.LineTag == "电信 163" || h.LineTag == "Cogent" || h.LineTag == "教育网 CERNET" {
			return h.LineTag
		}
	}
	return ""
}

// extractRealPublicIP extracts the true public IP behind Nginx / Cloudflare / reverse proxies.
func extractRealPublicIP(c *gin.Context) string {
	candidates := make([]string, 0, 4)
	if xri := strings.TrimSpace(c.GetHeader("X-Real-IP")); xri != "" {
		candidates = append(candidates, xri)
	}
	if xff := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			if p := strings.TrimSpace(part); p != "" {
				candidates = append(candidates, p)
			}
		}
	}
	if cip := strings.TrimSpace(c.ClientIP()); cip != "" {
		candidates = append(candidates, cip)
	}
	var ipv6Fallback string
	for _, raw := range candidates {
		ip := net.ParseIP(raw)
		if ip == nil || isPrivateOrLAN(raw) {
			continue
		}
		if ip.To4() != nil {
			return ip.String()
		}
		if ipv6Fallback == "" {
			ipv6Fallback = ip.String()
		}
	}
	return ipv6Fallback
}

var probeISPEnrichGuard = struct {
	mu   sync.Mutex
	seen map[uint]time.Time
}{
	seen: map[uint]time.Time{},
}

func normalizeProbeISPLabel(raw string) string {
	low := strings.ToLower(raw)
	switch {
	case strings.Contains(low, "chinanet") || strings.Contains(low, "chinatelecom") || strings.Contains(low, "china telecom") || strings.Contains(raw, "电信") || strings.Contains(low, "as4134") || strings.Contains(low, "as4809"):
		return "中国电信"
	case strings.Contains(low, "unicom") || strings.Contains(raw, "联通") || strings.Contains(low, "as4837") || strings.Contains(low, "as9929"):
		return "中国联通"
	case strings.Contains(low, "china mobile") || strings.Contains(low, "cmnet") || strings.Contains(raw, "移动") || strings.Contains(low, "as9808") || strings.Contains(low, "as56040"):
		return "中国移动"
	case strings.Contains(low, "cernet") || strings.Contains(low, "as4538") || strings.Contains(raw, "教育网") || strings.Contains(low, "education"):
		return "教育网"
	case strings.Contains(raw, "广电") || strings.Contains(low, "cbn"):
		return "中国广电"
	default:
		return ""
	}
}

func enrichProbeISPAsync(probeID uint, ip string) {
	probeISPEnrichGuard.mu.Lock()
	if last, ok := probeISPEnrichGuard.seen[probeID]; ok && time.Since(last) < 10*time.Minute {
		probeISPEnrichGuard.mu.Unlock()
		return
	}
	probeISPEnrichGuard.seen[probeID] = time.Now()
	probeISPEnrichGuard.mu.Unlock()

	hops := []userTraceHop{{TTL: 1, IP: ip, RTT: 1}}
	enrichTraceHops(hops)
	if hops[0].ISP == "" && hops[0].ASN == "" {
		return
	}
	label := normalizeProbeISPLabel(hops[0].ISP + " " + hops[0].ASN)
	if label == "" {
		label = strings.TrimSpace(hops[0].ISP)
		if len(label) > 24 {
			label = label[:24]
		}
	}
	if label != "" {
		models.DB.Model(&models.Probe{}).Where("id = ? AND (isp = '' OR isp IS NULL)", probeID).Update("isp", label)
	}
}

func matchReturnTraceServer(host string, port int) *models.ProxyServer {
	var servers []models.ProxyServer
	if models.DB.Where("removing = ?", false).Find(&servers).Error != nil || len(servers) == 0 {
		return nil
	}
	onlineByID := map[uint]*models.ProxyServer{}
	for i := range servers {
		s := &servers[i]
		if s.LastSeenAt != nil && time.Since(*s.LastSeenAt) < 2*time.Minute {
			onlineByID[s.ID] = s
		}
	}
	if len(onlineByID) == 0 {
		return nil
	}

	trimmedHost := strings.TrimSpace(host)
	// 1. Exact PublicHost match.
	for _, s := range onlineByID {
		if strings.EqualFold(strings.TrimSpace(s.PublicHost), trimmedHost) {
			return s
		}
	}

	// 2. Match via ProxyInbound SNI / Host + Port (handles custom domains like hy2.subl.in).
	if port > 0 && trimmedHost != "" {
		var inbs []models.ProxyInbound
		if models.DB.Where("port = ?", port).Find(&inbs).Error == nil {
			for _, inb := range inbs {
				if strings.EqualFold(strings.TrimSpace(inb.SNI), trimmedHost) ||
					strings.EqualFold(strings.TrimSpace(inb.Host), trimmedHost) {
					if srv, ok := onlineByID[inb.ServerID]; ok {
						return srv
					}
				}
			}
		}
	}

	// 3. Resolved IP match (handles node domain -> server PublicHost IP).
	hostIPs := map[string]bool{}
	if ip := net.ParseIP(trimmedHost); ip != nil {
		hostIPs[ip.String()] = true
	} else if ips, err := net.LookupIP(trimmedHost); err == nil {
		for _, ip := range ips {
			hostIPs[ip.String()] = true
		}
	}
	if len(hostIPs) > 0 {
		for _, s := range onlineByID {
			sh := strings.TrimSpace(s.PublicHost)
			if sh == "" {
				continue
			}
			if ip := net.ParseIP(sh); ip != nil && hostIPs[ip.String()] {
				return s
			}
		}
	}
	return nil
}

// ProbeBidirectionalTrace 并行发起「本机去程（User Agent → 节点）」与「服务器回程（代理 Agent → 本机公网 IP）」。
func ProbeBidirectionalTrace(c *gin.Context) {
	var req struct {
		NodeID  int  `json:"nodeId"`
		ProbeID uint `json:"probeId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "节点 ID 无效"})
		return
	}
	var n models.Node
	if err := models.DB.First(&n, req.NodeID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "节点不存在"})
		return
	}
	host, port := node.ExtractServerHost(n.Link)
	if host == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "无法解析节点主机地址"})
		return
	}

	targetProbe, err := pickOnlineUserProbe(req.ProbeID)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}
	if targetProbe == nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "没有在线的用户 Agent"})
		return
	}

	// Find matching online self-hosted ProxyServer for return-path tracing.
	matchedServer := matchReturnTraceServer(host, port)

	clientPubIP := strings.TrimSpace(targetProbe.PublicIP)
	if clientPubIP == "" || isPrivateOrLAN(clientPubIP) {
		clientPubIP = extractRealPublicIP(c)
	}

	var (
		wg          sync.WaitGroup
		outHops     []userTraceHop
		outErr      string
		retHops     []userTraceHop
		retErr      string
		retServer   string
	)

	// 1. Outbound (User Agent -> Node)
	wg.Add(1)
	go func() {
		defer wg.Done()
		jobID, _, ok := probeHub.dispatchJob(targetProbe.ID, queuedProbeJob{
			Action:     "trace",
			TargetHost: host,
		}, true)
		if !ok {
			outErr = "用户 Agent 繁忙"
			return
		}
		defer probeHub.cleanupDone(jobID)

		resCh := make(chan userTraceResult, 1)
		traceResultsHub.mu.Lock()
		traceResultsHub.waiters[jobID] = resCh
		traceResultsHub.mu.Unlock()
		defer func() {
			traceResultsHub.mu.Lock()
			delete(traceResultsHub.waiters, jobID)
			traceResultsHub.mu.Unlock()
		}()

		select {
		case res := <-resCh:
			outHops = res.Hops
			outErr = res.Error
			enrichTraceHops(outHops)
		case <-time.After(24 * time.Second):
			outErr = "去程追踪超时"
		case <-c.Request.Context().Done():
			outErr = "已取消"
		}
	}()

	// 2. Return (Server Agent -> Local Public IP)
	if matchedServer != nil && clientPubIP != "" {
		retServer = matchedServer.Name
		family := "4"
		if ip := net.ParseIP(clientPubIP); ip != nil && ip.To4() == nil {
			family = "6"
		}
		wg.Add(1)
		go func(srv models.ProxyServer, dstIP, fam string) {
			defer wg.Done()
			now := time.Now()
			run := models.NetworkTraceRun{
				Protocol:  "icmp",
				Port:      443,
				Family:    fam,
				Target:    dstIP,
				CreatedAt: now,
			}
			if err := models.DB.Create(&run).Error; err != nil {
				retErr = "创建回程追踪任务失败"
				return
			}
			view := models.NetworkTraceView{
				RunID:         run.ID,
				AgentServerID: srv.ID,
				SourceName:    srv.Name,
				Status:        "queued",
				CreatedAt:     now,
			}
			if err := models.DB.Create(&view).Error; err != nil {
				retErr = "创建回程视图失败"
				return
			}
			deadline := time.Now().Add(24 * time.Second)
			for time.Now().Before(deadline) {
				if c.Request.Context().Err() != nil {
					retErr = "已取消"
					return
				}
				var cur models.NetworkTraceView
				if models.DB.First(&cur, view.ID).Error == nil {
					if cur.Status == "completed" || cur.Status == "done" || cur.Status == "failed" || cur.Status == "unsupported" || cur.Status == "timeout" {
						if cur.Error != "" && cur.Output == "" {
							retErr = cur.Error
							return
						}
						parsed, perr := tracecore.ParseNextTrace(cur.Output)
						if perr != nil || len(parsed) == 0 {
							parsed = tracecore.Parse(cur.Output)
						}
						hops := make([]userTraceHop, 0, len(parsed))
						for _, h := range parsed {
							rtt := -1
							if h.RTT != nil && *h.RTT > 0 {
								rtt = max(int(math.Round(*h.RTT)), 1)
							}
							hops = append(hops, userTraceHop{
								TTL:      h.TTL,
								IP:       h.IP,
								RTT:      rtt,
								Timeout:  !h.Responded || h.IP == "",
								ASN:      h.ASN,
								ISP:      h.ISP,
								Location: strings.TrimSpace(strings.Join([]string{h.Country, h.City}, " ")),
							})
						}
						enrichTraceHops(hops)
						retHops = hops
						return
					}
				}
				time.Sleep(500 * time.Millisecond)
			}
			retErr = "服务器回程追踪等待超时"
		}(*matchedServer, clientPubIP, family)
	} else if matchedServer == nil {
		retErr = "该节点非自建服务器或代理 Agent 离线，仅执行本机去程追踪"
	} else {
		retErr = "尚未识别本机公网 IP（请确认用户 Agent 在线后重试）"
	}

	wg.Wait()

	outLine := summarizePathQuality(outHops)
	retLine := summarizePathQuality(retHops)
	verdict := "双向路由追踪完成"
	switch {
	case outLine != "" && retLine != "":
		if outLine == retLine {
			verdict = fmt.Sprintf("去程 %s ↔ 回程 %s（双向对称）", outLine, retLine)
		} else {
			verdict = fmt.Sprintf("去程 %s ↔ 回程 %s（去回不对称）", outLine, retLine)
		}
	case outLine != "":
		verdict = fmt.Sprintf("去程 %s", outLine)
	case retLine != "":
		verdict = fmt.Sprintf("回程 %s", retLine)
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"nodeName":     n.Name,
			"nodeHost":     host,
			"probeName":    targetProbe.Name,
			"probeIsp":     targetProbe.ISP,
			"clientIp":     clientPubIP,
			"returnServer": retServer,
			"outbound": gin.H{
				"hops":  outHops,
				"line":  outLine,
				"error": outErr,
			},
			"returnPath": gin.H{
				"hops":  retHops,
				"line":  retLine,
				"error": retErr,
			},
			"verdict": verdict,
		},
		"msg": verdict,
	})
}

// ProbeDiagnoseLocal 节点诊断第一段：从在线用户 Agent 物理直连实测单个节点入口连通性与延迟。
func ProbeDiagnoseLocal(c *gin.Context) {
	var req struct {
		NodeID  int  `json:"nodeId"`
		ProbeID uint `json:"probeId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "节点 ID 无效"})
		return
	}

	target, err := pickOnlineUserProbe(req.ProbeID)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}
	if target == nil {
		c.JSON(200, gin.H{
			"code": "00000",
			"data": gin.H{"available": false, "reason": "当前无在线用户 Agent"},
			"msg":  "无在线用户 Agent",
		})
		return
	}

	nodes, err := buildProbeNodes([]int{req.NodeID})
	if err != nil || len(nodes) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "节点不存在或地址无效"})
		return
	}
	jobID, doneCh, ok := probeHub.dispatch(target.ID, nodes, true)
	if !ok {
		c.JSON(409, gin.H{"code": "40900", "msg": "用户 Agent 繁忙，请稍后再试"})
		return
	}
	defer probeHub.cleanupDone(jobID)

	select {
	case <-doneCh:
	case <-time.After(12 * time.Second):
		c.JSON(200, gin.H{
			"code": "00000",
			"data": gin.H{"available": true, "probeName": target.Name, "success": false, "error": "本机实测超时"},
			"msg":  "本机实测超时",
		})
		return
	case <-c.Request.Context().Done():
		return
	}

	var latest models.ProbeResult
	_ = models.DB.Where("probe_id = ? AND node_id = ?", target.ID, req.NodeID).
		Order("checked_at desc, id desc").First(&latest).Error

	ipv6Mismatch := target.HasIPv6 != nil && !*target.HasIPv6 && strings.Contains(nodes[0].Host, ":") && !latest.Success
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"available":    true,
			"probeName":    target.Name,
			"probeIsp":     target.ISP,
			"hasIpv6":      target.HasIPv6,
			"ipv6Mismatch": ipv6Mismatch,
			"host":         nodes[0].Host,
			"port":         nodes[0].Port,
			"kind":         latest.Kind,
			"success":      latest.Success,
			"rtt":          latest.RTT,
			"error":        latest.Error,
		},
		"msg": "ok",
	})
}
