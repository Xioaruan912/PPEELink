package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ppeelink/models"
	"ppeelink/node"

	"github.com/gin-gonic/gin"
)

type userTraceHop struct {
	TTL      int    `json:"ttl"`
	IP       string `json:"ip"`
	RTT      int    `json:"rtt"`
	Timeout  bool   `json:"timeout"`
	ASN      string `json:"asn,omitempty"`
	ISP      string `json:"isp,omitempty"`
	Location string `json:"location,omitempty"`
	LineTag  string `json:"lineTag,omitempty"`
	LineTone string `json:"lineTone,omitempty"` // ok | info | warn | muted
}

type userTraceResult struct {
	Host  string         `json:"host"`
	Hops  []userTraceHop `json:"hops"`
	Error string         `json:"error,omitempty"`
}

var traceResultsHub = struct {
	mu      sync.Mutex
	waiters map[string]chan userTraceResult
}{
	waiters: map[string]chan userTraceResult{},
}

var hopGeoCache = struct {
	mu   sync.RWMutex
	data map[string]hopGeoInfo
}{
	data: map[string]hopGeoInfo{},
}

type hopGeoInfo struct {
	ASN      string
	ISP      string
	Location string
	LineTag  string
	LineTone string
}

// ProbeTrace 管理端：向在线用户 Agent 下发一次按需去程路由追踪（绕过 TUN）。
func ProbeTrace(c *gin.Context) {
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
	host, _ := node.ExtractServerHost(n.Link)
	if host == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "无法解析节点主机地址"})
		return
	}

	probes, err := models.GetProbes(true)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}
	var target, fallback *models.Probe
	for i := range probes {
		p := &probes[i]
		if req.ProbeID > 0 && p.ID != req.ProbeID {
			continue
		}
		if probeHub.isWaiting(p.ID) {
			target = p
			break
		}
		if fallback == nil && isProbeRecentlyOnline(p) {
			fallback = p
		}
	}
	if target == nil {
		target = fallback
	}
	if target == nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "没有在线的用户 Agent"})
		return
	}

	jobID, _, ok := probeHub.dispatchJob(target.ID, queuedProbeJob{
		Action:     "trace",
		TargetHost: host,
	}, true)
	if !ok {
		c.JSON(409, gin.H{"code": "40900", "msg": "用户 Agent 繁忙，请稍后再试"})
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
		if res.Error != "" && len(res.Hops) == 0 {
			c.JSON(500, gin.H{"code": "50000", "msg": res.Error})
			return
		}
		enrichTraceHops(res.Hops)
		c.JSON(200, gin.H{
			"code": "00000",
			"data": gin.H{
				"probeName": target.Name,
				"host":      res.Host,
				"hops":      res.Hops,
				"error":     res.Error,
			},
			"msg": "去程路由追踪完成",
		})
	case <-time.After(26 * time.Second):
		c.JSON(504, gin.H{"code": "50400", "msg": "路由追踪超时，请确认本机用户 Agent 已升级至 v1.3"})
	case <-c.Request.Context().Done():
		return
	}
}

// UserProbeTraceReport 用户 Agent 回报去程路由追踪结果。
func UserProbeTraceReport(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
		return
	}
	var req struct {
		JobID string         `json:"jobId"`
		Host  string         `json:"host"`
		Hops  []userTraceHop `json:"hops"`
		Error string         `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "格式错误"})
		return
	}
	traceResultsHub.mu.Lock()
	ch, ok := traceResultsHub.waiters[req.JobID]
	if ok {
		delete(traceResultsHub.waiters, req.JobID)
	}
	traceResultsHub.mu.Unlock()
	if ok {
		select {
		case ch <- userTraceResult{Host: req.Host, Hops: req.Hops, Error: req.Error}:
		default:
		}
		close(ch)
	}
	probeHub.complete(req.JobID)
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

func isPrivateOrLAN(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true // CGNAT 100.64.0.0/10
	}
	return false
}

func classifyCarrierLine(ipStr, asText, ispText string) (string, string) {
	if isPrivateOrLAN(ipStr) {
		return "内网", "muted"
	}
	combined := strings.ToLower(asText + " " + ispText)
	switch {
	case strings.HasPrefix(ipStr, "59.43.") || strings.Contains(combined, "as4809"):
		return "CN2 GIA", "ok"
	case strings.Contains(combined, "as58807"):
		return "CMIN2", "ok"
	case strings.Contains(combined, "as9929"):
		return "联通 9929", "ok"
	case strings.Contains(combined, "as58453") || strings.Contains(combined, "as9808"):
		return "移动 CMI", "info"
	case strings.Contains(combined, "as4837"):
		return "联通 4837", "info"
	case strings.HasPrefix(ipStr, "202.97.") || strings.Contains(combined, "as4134"):
		return "电信 163", "warn"
	case strings.Contains(combined, "as2914") || strings.Contains(combined, "ntt"):
		return "NTT", "info"
	case strings.Contains(combined, "as2497") || strings.Contains(combined, "iij"):
		return "IIJ", "info"
	case strings.Contains(combined, "as17676") || strings.Contains(combined, "softbank"):
		return "SoftBank", "info"
	case strings.Contains(combined, "as174") || strings.Contains(combined, "cogent"):
		return "Cogent", "warn"
	case strings.Contains(combined, "as13335") || strings.Contains(combined, "cloudflare"):
		return "Cloudflare", "info"
	default:
		return "", ""
	}
}

func enrichTraceHops(hops []userTraceHop) {
	var toQuery []string
	seen := map[string]bool{}
	for i := range hops {
		ip := strings.TrimSpace(hops[i].IP)
		if ip == "" || hops[i].Timeout {
			continue
		}
		if isPrivateOrLAN(ip) {
			hops[i].Location = "局域网 / 运营商内网"
			hops[i].LineTag = "内网"
			hops[i].LineTone = "muted"
			continue
		}
		hopGeoCache.mu.RLock()
		info, ok := hopGeoCache.data[ip]
		hopGeoCache.mu.RUnlock()
		if ok {
			hops[i].ASN = info.ASN
			hops[i].ISP = info.ISP
			hops[i].Location = info.Location
			hops[i].LineTag = info.LineTag
			hops[i].LineTone = info.LineTone
			continue
		}
		if !seen[ip] {
			seen[ip] = true
			toQuery = append(toQuery, ip)
		}
	}

	if len(toQuery) > 0 {
		if len(toQuery) > 40 {
			toQuery = toQuery[:40]
		}
		raw, _ := json.Marshal(toQuery)
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Post("http://ip-api.com/batch?fields=query,status,country,city,isp,as&lang=zh-CN", "application/json", bytes.NewReader(raw))
		if err == nil {
			defer resp.Body.Close()
			var batch []struct {
				Query   string `json:"query"`
				Status  string `json:"status"`
				Country string `json:"country"`
				City    string `json:"city"`
				ISP     string `json:"isp"`
				AS      string `json:"as"`
			}
			if json.NewDecoder(resp.Body).Decode(&batch) == nil {
				hopGeoCache.mu.Lock()
				for _, item := range batch {
					if item.Status != "success" || item.Query == "" {
						continue
					}
					loc := strings.TrimSpace(item.Country)
					if item.City != "" && item.City != item.Country {
						loc = strings.TrimSpace(loc + " " + item.City)
					}
					asn := item.AS
					if fields := strings.Fields(asn); len(fields) > 0 {
						asn = fields[0]
					}
					tag, tone := classifyCarrierLine(item.Query, item.AS, item.ISP)
					hopGeoCache.data[item.Query] = hopGeoInfo{
						ASN:      asn,
						ISP:      item.ISP,
						Location: loc,
						LineTag:  tag,
						LineTone: tone,
					}
				}
				hopGeoCache.mu.Unlock()
			}
		}
		for i := range hops {
			ip := strings.TrimSpace(hops[i].IP)
			if ip == "" || hops[i].Timeout || hops[i].LineTag == "内网" {
				continue
			}
			hopGeoCache.mu.RLock()
			info, ok := hopGeoCache.data[ip]
			hopGeoCache.mu.RUnlock()
			if ok {
				hops[i].ASN = info.ASN
				hops[i].ISP = info.ISP
				hops[i].Location = info.Location
				hops[i].LineTag = info.LineTag
				hops[i].LineTone = info.LineTone
			} else if tag, tone := classifyCarrierLine(ip, "", ""); tag != "" {
				hops[i].LineTag = tag
				hops[i].LineTone = tone
			}
		}
	}
}

// ProbeApplySort 根据用户 Agent 真实延迟重排所有订阅的 node_order（可达低延迟靠前，不可达靠后）。
func ProbeApplySort(c *gin.Context) {
	var req struct {
		ProbeID uint `json:"probeId"`
	}
	_ = c.ShouldBindJSON(&req)

	nodes, err := models.GetNodeList()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点失败"})
		return
	}
	q := models.DB.Where("probe_id > 0")
	if req.ProbeID > 0 {
		q = q.Where("probe_id = ?", req.ProbeID)
	}
	var rows []models.ProbeResult
	_ = q.Order("checked_at desc, id desc").Find(&rows).Error

	bestRTT := make(map[int]int)
	for _, r := range rows {
		if _, seen := bestRTT[r.NodeID]; seen {
			continue
		}
		if r.Success && r.RTT > 0 {
			bestRTT[r.NodeID] = r.RTT
		} else {
			bestRTT[r.NodeID] = 999999
		}
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		ri, oki := bestRTT[nodes[i].ID]
		if !oki {
			ri = 888888
		}
		rj, okj := bestRTT[nodes[j].ID]
		if !okj {
			rj = 888888
		}
		return ri < rj
	})

	rank := make(map[string]int, len(nodes))
	for idx, n := range nodes {
		rank[n.Name] = idx
	}

	var subs []models.Subcription
	if err := models.DB.Preload("Nodes").Preload("GroupRefs.Nodes").Find(&subs).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取订阅失败"})
		return
	}
	updated := 0
	for _, sub := range subs {
		seen := map[string]bool{}
		var names []string
		for _, n := range sub.Nodes {
			if !seen[n.Name] {
				seen[n.Name] = true
				names = append(names, n.Name)
			}
		}
		for _, g := range sub.GroupRefs {
			for _, n := range g.Nodes {
				if !seen[n.Name] {
					seen[n.Name] = true
					names = append(names, n.Name)
				}
			}
		}
		if len(names) == 0 {
			continue
		}
		sort.SliceStable(names, func(i, j int) bool {
			ri, oki := rank[names[i]]
			if !oki {
				ri = 999999
			}
			rj, okj := rank[names[j]]
			if !okj {
				rj = 999999
			}
			return ri < rj
		})
		raw, _ := json.Marshal(names)
		if err := models.DB.Model(&models.Subcription{}).Where("id = ?", sub.ID).Update("node_order", string(raw)).Error; err == nil {
			updated++
		}
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{"updatedSubscriptions": updated},
		"msg":  fmt.Sprintf("已按本机实测延迟重排 %d 个订阅的节点顺序", updated),
	})
}
