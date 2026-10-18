package api

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"ppeelink/models"
	"ppeelink/node"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// nodeBatchRequest 批量操作节点。Preview 为真时只返回改动预览，不落库。
type nodeBatchRequest struct {
	IDs         []int           `json:"ids"`
	Action      string          `json:"action"` // rename / emoji-add / emoji-remove / flags / chain
	Pattern     string          `json:"pattern"`
	Replacement string          `json:"replacement"`
	Flags       *node.NodeFlags `json:"flags"`
	RelayID     int             `json:"relayId"` // action=chain：中转节点 ID，0 表示取消
	Preview     bool            `json:"preview"`
}

type nodeChange struct {
	ID     int    `json:"id"`
	Before string `json:"before"`
	After  string `json:"after"`
	Link   string `json:"-"`
	Flags  string `json:"-"`
}

var leadingFlag = regexp.MustCompile(`^(?:[\x{1F1E6}-\x{1F1FF}]{2}\s*)+`)

// renameNode 按正则替换名称；正则为空时视为整体替换为 replacement。
func renameNode(name string, re *regexp.Regexp, replacement string) string {
	if re == nil {
		return replacement
	}
	return strings.TrimSpace(re.ReplaceAllString(name, replacement))
}

// withFlagEmoji 先去掉已有国旗，再按国家补上（GeoIP 优先，名称兜底）。
func withFlagEmoji(p node.Outbound) string {
	bare := strings.TrimSpace(leadingFlag.ReplaceAllString(p.Name, ""))
	code := p.CountryCode()
	if code == "" {
		code = node.CountryFromName(bare)
	}
	if flag := node.FlagEmoji(code); flag != "" {
		return flag + " " + bare
	}
	return bare
}

// NodeBatch 批量改名、加减国旗或设置 UDP/TFO/跳过证书校验。
func NodeBatch(c *gin.Context) {
	var req nodeBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "请选择节点"})
		return
	}
	var re *regexp.Regexp
	if req.Action == "rename" && req.Pattern != "" {
		var err error
		if re, err = regexp.Compile(req.Pattern); err != nil {
			c.JSON(400, gin.H{"code": "40000", "msg": "正则无效: " + err.Error()})
			return
		}
	}
	if req.Action == "rename" && re == nil && strings.TrimSpace(req.Replacement) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "请填写匹配正则或新名称"})
		return
	}
	if req.Action == "chain" && req.RelayID != 0 {
		var relay models.Node
		if err := models.DB.First(&relay, req.RelayID).Error; err != nil {
			c.JSON(400, gin.H{"code": "40000", "msg": "中转节点不存在"})
			return
		}
	}
	var nodes []models.Node
	if err := models.DB.Where("id IN ?", req.IDs).Order("id asc").Find(&nodes).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点失败"})
		return
	}
	changes := []nodeChange{}
	for _, n := range nodes {
		change := nodeChange{ID: n.ID, Before: n.Name, After: n.Name, Link: n.Link, Flags: n.Flags}
		switch req.Action {
		case "rename", "emoji-add", "emoji-remove":
			p, err := node.ParseOutbound(n.Link)
			if err != nil {
				continue // 无法解析的链接不改名，避免名称与链接不一致
			}
			p.Name = n.Name
			switch req.Action {
			case "rename":
				change.After = renameNode(n.Name, re, req.Replacement)
			case "emoji-add":
				change.After = withFlagEmoji(p)
			case "emoji-remove":
				change.After = strings.TrimSpace(leadingFlag.ReplaceAllString(n.Name, ""))
			}
			if change.After == "" || change.After == n.Name {
				continue
			}
			change.Link = p.WithName(change.After).Link
		case "flags":
			if req.Flags == nil {
				c.JSON(400, gin.H{"code": "40000", "msg": "缺少开关设置"})
				return
			}
			merged := decodeNodeFlags(n.Flags)
			merged.UDP, merged.TFO, merged.SkipCertVerify = req.Flags.UDP, req.Flags.TFO, req.Flags.SkipCertVerify
			change.Flags = encodeNodeFlags(merged)
			if change.Flags == n.Flags {
				continue
			}
		case "chain":
			if req.RelayID == n.ID {
				continue // 节点不能经由自己中转
			}
			merged := decodeNodeFlags(n.Flags)
			merged.DialerProxyID = req.RelayID
			change.Flags = encodeNodeFlags(merged)
			if change.Flags == n.Flags {
				continue
			}
		default:
			c.JSON(400, gin.H{"code": "40000", "msg": "不支持的操作"})
			return
		}
		changes = append(changes, change)
	}
	if req.Preview || len(changes) == 0 {
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{"changes": changes}, "msg": "预览"})
		return
	}
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		for _, ch := range changes {
			if err := tx.Model(&models.Node{}).Where("id = ?", ch.ID).
				Updates(map[string]any{"name": ch.After, "link": ch.Link, "flags": ch.Flags}).Error; err != nil {
				return err
			}
			if ch.Before != ch.After {
				if err := renameInNodeOrders(tx, ch.Before, ch.After); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败: " + err.Error()})
		return
	}
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"changes": changes}, "msg": fmt.Sprintf("已更新 %d 个节点", len(changes))})
}

// renameInNodeOrders 订阅的节点顺序按名称保存，改名后同步替换。
func renameInNodeOrders(tx *gorm.DB, before, after string) error {
	var subs []models.Subcription
	if err := tx.Where("node_order LIKE ?", "%"+before+"%").Find(&subs).Error; err != nil {
		return err
	}
	for _, sub := range subs {
		parts := strings.Split(sub.NodeOrder, ",")
		changed := false
		for i, p := range parts {
			if strings.TrimSpace(p) == before {
				parts[i] = after
				changed = true
			}
		}
		if changed {
			if err := tx.Model(&models.Subcription{}).Where("id = ?", sub.ID).
				Update("node_order", strings.Join(parts, ",")).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// dedupeIdentity 去重依据：协议 + 服务器 + 端口（忽略名称）。
func dedupeIdentity(link string) string {
	p, err := node.ParseOutbound(link)
	if err != nil || p.Server == "" {
		return ""
	}
	return strings.ToLower(fmt.Sprintf("%s|%s|%d", p.Type, p.Server, p.Port))
}

type dedupeGroup struct {
	Keep    models.Node   `json:"keep"`
	Removed []models.Node `json:"removed"`
}

// NodeDedupe 找出（或删除）重复节点：保留最早的一个，并把分组与订阅引用转移给它。
func NodeDedupe(c *gin.Context) {
	var req struct {
		Preview bool `json:"preview"`
	}
	_ = c.ShouldBindJSON(&req)
	var nodes []models.Node
	if err := models.DB.Order("id asc").Find(&nodes).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点失败"})
		return
	}
	byKey := map[string][]models.Node{}
	keys := []string{}
	for _, n := range nodes {
		key := dedupeIdentity(n.Link)
		if key == "" {
			continue
		}
		if _, ok := byKey[key]; !ok {
			keys = append(keys, key)
		}
		byKey[key] = append(byKey[key], n)
	}
	sort.Strings(keys)
	groups := []dedupeGroup{}
	removed := 0
	for _, key := range keys {
		list := byKey[key]
		if len(list) < 2 {
			continue
		}
		groups = append(groups, dedupeGroup{Keep: list[0], Removed: list[1:]})
		removed += len(list) - 1
	}
	if req.Preview || removed == 0 {
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{"groups": groups, "removed": removed}, "msg": "预览"})
		return
	}
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		for _, g := range groups {
			for _, dup := range g.Removed {
				for _, stmt := range []string{
					"INSERT OR IGNORE INTO group_node_nodes (node_id, group_node_id) SELECT ?, group_node_id FROM group_node_nodes WHERE node_id = ?",
					"INSERT OR IGNORE INTO subcription_nodes (subcription_id, node_id) SELECT subcription_id, ? FROM subcription_nodes WHERE node_id = ?",
				} {
					if err := tx.Exec(stmt, g.Keep.ID, dup.ID).Error; err != nil {
						return err
					}
				}
				for _, table := range []string{"group_node_nodes", "subcription_nodes"} {
					if err := tx.Exec("DELETE FROM "+table+" WHERE node_id = ?", dup.ID).Error; err != nil {
						return err
					}
				}
				if err := tx.Delete(&models.Node{}, dup.ID).Error; err != nil {
					return err
				}
				if dup.Name != g.Keep.Name {
					if err := renameInNodeOrders(tx, dup.Name, g.Keep.Name); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "去重失败: " + err.Error()})
		return
	}
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"groups": groups, "removed": removed}, "msg": fmt.Sprintf("已移除 %d 个重复节点", removed)})
}
