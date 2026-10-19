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
	RelayID        int       `json:"relayId,omitempty"` // 链式代理的中转节点
	UDP            bool      `json:"udp,omitempty"`
	TFO            bool      `json:"tfo,omitempty"`
	SkipCertVerify bool      `json:"skipCertVerify,omitempty"`
	// 暂不下发的原因（为空表示正常下发）
	Blocked *deliveryBlock `json:"blocked,omitempty"`
	rtt     int            // 本次连接耗时，-1 失败；仅用于记录与健康事件
}

// qualityCheckConcurrency 节点 TCP 质量检测的最大并发连接数。
const qualityCheckConcurrency = 16

// overview 缓存（5 分钟；节点变更时立即失效）
type overviewCache struct {
	mu      sync.Mutex
	data    []NodeOverviewItem
	expires time.Time
}

var oCache = &overviewCache{}

// InvalidateOverview 节点数据变更后调用，使概览缓存立即失效
func InvalidateOverview() {
	oCache.mu.Lock()
	oCache.data = nil
	oCache.expires = time.Time{}
	oCache.mu.Unlock()
}

// NodeOverview 返回所有节点概览：国家（名称优先，GeoIP 兜底）+ 有效性 + 分组。
// GET /api/v1/nodes/overview?refresh=1 立即重新检测
func NodeOverview(c *gin.Context) {
	oCache.mu.Lock()
	if c.Query("refresh") != "1" && oCache.data != nil && time.Now().Before(oCache.expires) {
		data := oCache.data
		oCache.mu.Unlock()
		c.JSON(200, gin.H{"code": "00000", "data": withBlocks(data), "msg": "节点概览（缓存）"})
		return
	}
	oCache.mu.Unlock()

	items, err := CollectNodeQuality()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "获取节点列表失败"})
		return
	}

	oCache.mu.Lock()
	oCache.data = items
	oCache.expires = time.Now().Add(5 * time.Minute)
	oCache.mu.Unlock()

	c.JSON(200, gin.H{"code": "00000", "data": withBlocks(items), "msg": "节点概览"})
}

// withBlocks 复制概览并附上此刻的暂不下发原因（暂停、额度变化随时生效，不受缓存影响）。
func withBlocks(items []NodeOverviewItem) []NodeOverviewItem {
	blocks := undeliverableNodes(time.Now())
	out := make([]NodeOverviewItem, len(items))
	for i, item := range items {
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
	// 并发上限避免节点很多时瞬间向机场发起大量连接（易被判定为扫描）。
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
				item.RelayID, item.UDP, item.TFO, item.SkipCertVerify = f.DialerProxyID, f.UDP, f.TFO, f.SkipCertVerify
			}
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
	stats, err := models.GetNodeQualityStats(time.Now().Add(-24 * time.Hour))
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点状态失败"})
		return
	}
	// 只统计当前可见的节点（已删除或隐藏的节点在 24 小时内仍有检测记录）
	nodes, err := models.GetNodeList()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点失败"})
		return
	}
	total, valid := 0, 0
	for _, n := range nodes {
		stat, ok := stats[n.ID]
		if !ok {
			continue
		}
		total++
		if stat.LastRtt >= 0 {
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
