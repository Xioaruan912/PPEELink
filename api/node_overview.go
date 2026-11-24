package api

import (
	"sync"
	"time"

	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/utils"

	"github.com/gin-gonic/gin"
)

// NodeOverviewItem 节点概览：国家、分组与有效性。
// 有效性由面板服务器 TCP 连接节点端口判断，连得上即有效；耗时只在内部使用，不对外展示。
type NodeOverviewItem struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Link           string    `json:"link"`
	Server         string    `json:"server"`
	Port           int       `json:"port"`
	Country        string    `json:"country"`
	CountryCode    string    `json:"countryCode"`
	Valid          bool      `json:"valid"`
	CheckedAt      time.Time `json:"checkedAt"`
	Groups         []string  `json:"groups"`
	Multiplier     float64   `json:"multiplier"`
	UDP            bool      `json:"udp,omitempty"`
	TFO            bool      `json:"tfo,omitempty"`
	SkipCertVerify bool      `json:"skipCertVerify,omitempty"`
	// ForceDeliver 为 true 时检测无效也照常下发
	ForceDeliver    bool `json:"forceDeliver,omitempty"`
	DeliveryEnabled bool `json:"deliveryEnabled"`
	// ProbePolicy 检测策略（保留字段兼容，已改由用户 Agent 实测为准）
	ProbePolicy string `json:"probePolicy"`
	// Probes 最近一次用户 Agent 实测结果
	Probes []ProbeStatus `json:"probes,omitempty"`
	// 用户 Agent（从用户主机出发）最近一次按需实测结果
	UserValid     *bool      `json:"userValid,omitempty"`
	UserRTT       *int       `json:"userRtt,omitempty"`
	UserKind      string     `json:"userKind,omitempty"`
	UserError     string     `json:"userError,omitempty"`
	UserCheckedAt *time.Time `json:"userCheckedAt,omitempty"`
	UserProbeName string     `json:"userProbeName,omitempty"`
	// 暂不下发的原因（为空表示正常下发）
	Blocked *deliveryBlock `json:"blocked,omitempty"`
	rtt     int            // 本次连接耗时，-1 失败；仅用于记录与健康事件
}

// ProbeStatus 单个探针对某个节点的最近一次检测结果。
type ProbeStatus struct {
	ID        uint       `json:"id"`
	Name      string     `json:"name"`
	Region    string     `json:"region"`
	Success   bool       `json:"success"`
	RTT       int        `json:"rtt"`
	Error     string     `json:"error,omitempty"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
}

// qualityCheckConcurrency 节点 TCP 质量检测的最大并发连接数。
const qualityCheckConcurrency = 16

// overview 缓存（5 分钟；节点变更时立即失效）
type overviewCache struct {
	mu       sync.Mutex
	data     []NodeOverviewItem
	expires  time.Time
	revision uint64
}

var oCache = &overviewCache{}

// InvalidateOverview 节点数据变更后调用，使概览缓存立即失效
func InvalidateOverview() {
	oCache.mu.Lock()
	oCache.data = nil
	oCache.expires = time.Time{}
	oCache.revision++
	oCache.mu.Unlock()
}

// probeResultTTL 检测结果有效期：agent 每 5 分钟回报，允许漏掉一轮。
const probeResultTTL = 12 * time.Minute

// applyProbeResults 合并用户 Agent（probe_id > 0）的按需实测结果。
// 不再使用海外服务器 Agent 互测结果判定用户可达性。
func applyProbeResults(items []NodeOverviewItem) {
	if len(items) == 0 {
		return
	}
	probes, err := models.GetProbes(true)
	if err != nil || len(probes) == 0 {
		return
	}
	probeByID := make(map[uint]models.Probe, len(probes))
	probeIDs := make([]uint, 0, len(probes))
	for _, p := range probes {
		probeByID[p.ID] = p
		probeIDs = append(probeIDs, p.ID)
	}
	nodeIDs := make([]int, 0, len(items))
	for _, item := range items {
		nodeIDs = append(nodeIDs, item.ID)
	}
	var rows []models.ProbeResult
	if err := models.DB.Where("probe_id IN ? AND node_id IN ?", probeIDs, nodeIDs).
		Order("checked_at desc, id desc").Find(&rows).Error; err != nil || len(rows) == 0 {
		return
	}
	latestByNode := make(map[int]models.ProbeResult, len(nodeIDs))
	for _, row := range rows {
		if _, exists := latestByNode[row.NodeID]; !exists {
			latestByNode[row.NodeID] = row
		}
	}
	for i := range items {
		item := &items[i]
		row, ok := latestByNode[item.ID]
		if !ok {
			continue
		}
		valid := row.Success
		rtt := row.RTT
		checkedAt := row.CheckedAt
		probeName := probeByID[row.ProbeID].Name
		item.UserValid = &valid
		item.UserRTT = &rtt
		item.UserKind = row.Kind
		item.UserError = row.Error
		item.UserCheckedAt = &checkedAt
		item.UserProbeName = probeName
		item.Valid = row.Success
		item.rtt = row.RTT
		item.Probes = []ProbeStatus{{
			ID:        row.ProbeID,
			Name:      probeName,
			Region:    probeByID[row.ProbeID].Region,
			Success:   row.Success,
			RTT:       row.RTT,
			Error:     row.Error,
			CheckedAt: &checkedAt,
		}}
	}
}

// overviewItems 返回概览，优先使用 5 分钟缓存；refresh 强制重新检测。
func overviewItems(refresh bool) ([]NodeOverviewItem, error) {
	oCache.mu.Lock()
	if !refresh && oCache.data != nil && time.Now().Before(oCache.expires) {
		data := oCache.data
		oCache.mu.Unlock()
		return data, nil
	}
	revision := oCache.revision
	oCache.mu.Unlock()

	items, err := CollectNodeQuality()
	if err != nil {
		return nil, err
	}
	oCache.mu.Lock()
	// A node may have been published/hidden while the network checks ran.
	// Do not restore an obsolete snapshot after its invalidation.
	if oCache.revision == revision {
		oCache.data = items
		oCache.expires = time.Now().Add(5 * time.Minute)
	}
	oCache.mu.Unlock()
	return items, nil
}

// NodeOverview 返回所有节点概览：国家（名称优先，GeoIP 兜底）+ 有效性 + 分组。
// GET /api/v1/nodes/overview?refresh=1 立即重新检测
func NodeOverview(c *gin.Context) {
	items, err := overviewItems(c.Query("refresh") == "1")
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "获取节点列表失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": withBlocks(items), "msg": "节点概览"})
}

// withBlocks 复制概览并附上此刻的暂不下发原因（暂停、额度变化随时生效，不受缓存影响）。
func withBlocks(items []NodeOverviewItem) []NodeOverviewItem {
	blocks := undeliverableNodes(time.Now())
	var disabled []int
	models.DB.Model(&models.Node{}).Where("delivery_disabled = ?", true).Pluck("id", &disabled)
	disabledIDs := make(map[int]bool, len(disabled))
	for _, id := range disabled {
		disabledIDs[id] = true
	}
	out := make([]NodeOverviewItem, len(items))
	for i, item := range items {
		item.DeliveryEnabled = !disabledIDs[item.ID]
		if b, ok := blocks[item.ID]; ok {
			b := b
			item.Blocked = &b
		}
		out[i] = item
	}
	return out
}

// CollectNodeQuality checks every node, persists the result and returns the
// enriched overview. It is shared by the HTTP endpoint and the scheduler.
func CollectNodeQuality() ([]NodeOverviewItem, error) {
	nodes, err := models.GetNodeList()
	if err != nil {
		return nil, err
	}

	// 逐节点解析服务器地址 + GeoIP 国家 + TCP 延迟。
	// 并发上限避免节点很多时瞬间发起大量连接（易被判定为扫描）。
	var wg sync.WaitGroup
	results := make([]NodeOverviewItem, len(nodes))
	slots := make(chan struct{}, qualityCheckConcurrency)
	for i, n := range nodes {
		wg.Add(1)
		go func(i int, n models.Node) {
			defer wg.Done()
			defer utils.RecoverPanic("node-overview")
			slots <- struct{}{}
			defer func() { <-slots }()
			item := NodeOverviewItem{
				ID: n.ID, Name: n.Name, Link: n.Link,
				rtt:        -1,
				Groups:     make([]string, 0),
				Multiplier: node.ParseMultiplier(n.Name),
			}
			if f := decodeNodeFlags(n.Flags); !f.Empty() {
				item.UDP, item.TFO, item.SkipCertVerify = f.UDP, f.TFO, f.SkipCertVerify
			}
			item.ForceDeliver = n.ForceDeliver
			item.ProbePolicy = n.ProbePolicy
			for _, g := range n.GroupNodes {
				item.Groups = append(item.Groups, g.Name)
			}
			host, port := node.ExtractServerHost(n.Link)
			if host != "" {
				item.Server = host
				item.Port = port
				item.CountryCode = node.NodeCountry(n.Name, host)
				item.Country = countryName(item.CountryCode)
				addr := host
				if port > 0 {
					addr = host + ":" + itoa(port)
				}
				item.rtt = node.TCPPing(addr, 3*time.Second)
			}
			item.Valid = item.rtt >= 0
			results[i] = item
		}(i, n)
	}
	wg.Wait()
	applyProbeResults(results)
	checkedAt := time.Now()
	for i := range results {
		results[i].CheckedAt = checkedAt
	}
	samples := make([]models.NodeQualitySample, 0, len(results))
	for _, item := range results {
		samples = append(samples, models.NodeQualitySample{
			NodeID: item.ID, Rtt: item.rtt, Success: item.Valid, CheckedAt: checkedAt,
		})
	}
	if err := models.RecordNodeQuality(samples); err != nil {
		return nil, err
	}
	processNodeHealthEvents(results)
	return results, nil
}

// NodeQualitySummary 首页用：有效与无效节点数（按最近一次检测）。
func NodeQualitySummary(c *gin.Context) {
	items, err := overviewItems(false)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点状态失败"})
		return
	}
	total, valid := len(items), 0
	for _, item := range items {
		if item.Valid {
			valid++
		}
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"total": total, "valid": valid, "invalid": total - valid,
	}, "msg": "节点有效性"})
}

// itoa 简易整数转字符串
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
