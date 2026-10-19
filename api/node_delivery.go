package api

import (
	"fmt"
	"strings"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

// 暂不下发：下面几种情况下节点不会出现在订阅、合集里，条件解除后自动恢复。
//   - 手动暂停到某个日期（例如节点本月流量用完）；
//   - 自建服务器本月流量超出上限，到重置日恢复；
//   - 上游订阅流量用完或已到期，下次同步拿到新额度后恢复；
//   - 面板连续多次检测无效（健康事件为 down），检测恢复后自动下发。

const (
	blockPaused   = "paused"
	blockServer   = "server-quota"
	blockUpstream = "upstream"
	blockHealth   = "health"
)

// deliveryBlock 节点暂不下发的原因。Until 为预计恢复时间，未知时为空。
type deliveryBlock struct {
	Kind   string     `json:"kind"`
	Reason string     `json:"reason"`
	Until  *time.Time `json:"until,omitempty"`
}

// undeliverableNodes 计算此刻所有暂不下发的节点。同一节点有多个原因时取最先命中的。
func undeliverableNodes(now time.Time) map[int]deliveryBlock {
	blocks := map[int]deliveryBlock{}
	add := func(id int, b deliveryBlock) {
		if _, ok := blocks[id]; !ok {
			blocks[id] = b
		}
	}

	var paused []models.Node
	models.DB.Select("id", "paused_until", "pause_note").Where("paused_until > ?", now).Find(&paused)
	for _, n := range paused {
		reason := "手动暂停"
		if n.PauseNote != "" {
			reason += "：" + n.PauseNote
		}
		add(n.ID, deliveryBlock{Kind: blockPaused, Reason: reason, Until: n.PausedUntil})
	}

	var servers []models.ProxyServer
	models.DB.Where("traffic_limit > 0").Find(&servers)
	for _, s := range servers {
		q := serverQuota(s, now)
		if !q.Over {
			continue
		}
		until := q.ResetAt
		for _, id := range groupNodeIDs(selfHostedGroupPrefix + s.Name) {
			add(id, deliveryBlock{Kind: blockServer, Reason: "自建服务器本月流量已用完", Until: &until})
		}
	}

	var upstreams []models.Subcription
	models.DB.Select("id", "name", "source_upload", "source_download", "source_total", "source_expire").
		Where("source_urls IS NOT NULL AND TRIM(source_urls) <> ''").Find(&upstreams)
	for _, u := range upstreams {
		reason := ""
		var until *time.Time
		switch {
		case u.SourceExpire != nil && now.After(*u.SourceExpire):
			reason = "上游订阅已到期"
		case u.SourceTotal > 0 && u.SourceUpload+u.SourceDownload >= u.SourceTotal:
			reason = "上游订阅流量已用完"
			until = u.SourceExpire
		}
		if reason == "" {
			continue
		}
		for _, id := range groupNodeIDs(u.Name) {
			add(id, deliveryBlock{Kind: blockUpstream, Reason: reason, Until: until})
		}
	}

	// 每个节点最近一次健康事件为 down，说明已连续多次检测无效
	var downs []models.NodeHealthEvent
	models.DB.Where("id IN (?)", models.DB.Model(&models.NodeHealthEvent{}).Select("MAX(id)").Group("node_id")).
		Where("type = ?", "down").Find(&downs)
	for _, e := range downs {
		add(e.NodeID, deliveryBlock{Kind: blockHealth, Reason: "连续多次检测无效"})
	}
	return blocks
}

func groupNodeIDs(groupName string) []int {
	var ids []int
	models.DB.Table("group_node_nodes").
		Joins("JOIN group_nodes ON group_nodes.id = group_node_nodes.group_node_id").
		Where("group_nodes.name = ? AND group_nodes.deleted_at IS NULL", groupName).
		Pluck("group_node_nodes.node_id", &ids)
	return ids
}

// excludedNode 订阅预览里展示的暂不下发节点。
type excludedNode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	deliveryBlock
}

// filterDeliverable 去掉暂不下发的节点。只因“检测无效”被拦下的节点在全部被拦时仍保留：
// 那多半是面板自己的网络出了问题，不应让订阅变空。
func filterDeliverable(nodes []models.Node, blocks map[int]deliveryBlock) ([]models.Node, []excludedNode) {
	kept := make([]models.Node, 0, len(nodes))
	excluded := []excludedNode{}
	healthOnly := []models.Node{}
	for _, n := range nodes {
		b, ok := blocks[n.ID]
		switch {
		case !ok:
			kept = append(kept, n)
		case b.Kind == blockHealth:
			healthOnly = append(healthOnly, n)
		default:
			excluded = append(excluded, excludedNode{ID: n.ID, Name: n.Name, deliveryBlock: b})
		}
	}
	if len(kept) == 0 && len(healthOnly) > 0 {
		return append(kept, healthOnly...), excluded
	}
	for _, n := range healthOnly {
		excluded = append(excluded, excludedNode{ID: n.ID, Name: n.Name, deliveryBlock: blocks[n.ID]})
	}
	return kept, excluded
}

type nodePauseRequest struct {
	IDs   []int  `json:"ids"`
	Until string `json:"until"` // 2006-01-02，空表示恢复下发
	Note  string `json:"note"`
}

// NodePause 暂停或恢复节点下发。暂停到 until 当天 0 点（服务器时区）自动恢复。
// POST /api/v1/nodes/pause
func NodePause(c *gin.Context) {
	var req nodePauseRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "请选择节点"})
		return
	}
	updates := map[string]any{"paused_until": nil, "pause_note": ""}
	msg := fmt.Sprintf("已恢复 %d 个节点的下发", len(req.IDs))
	if strings.TrimSpace(req.Until) != "" {
		until, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(req.Until), time.Local)
		if err != nil || !until.After(time.Now()) {
			c.JSON(400, gin.H{"code": "40000", "msg": "恢复日期需要是今天之后的日期"})
			return
		}
		note := strings.TrimSpace(req.Note)
		if len([]rune(note)) > 60 {
			note = string([]rune(note)[:60])
		}
		updates = map[string]any{"paused_until": until, "pause_note": note}
		msg = fmt.Sprintf("已暂停 %d 个节点，%s 起恢复下发", len(req.IDs), until.Format("01-02"))
	}
	if err := models.DB.Model(&models.Node{}).Where("id IN ?", req.IDs).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "msg": msg})
}
