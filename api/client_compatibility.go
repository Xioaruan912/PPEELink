package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"ppeelink/node"
)

func compatibilityInputs(nodes []models.Node) []node.CompatibilityInput {
	out := []node.CompatibilityInput{}
	for _, n := range nodes {
		for _, link := range expandNodeLink(n.Link) {
			out = append(out, node.CompatibilityInput{ID: n.ID, Name: n.Name, Link: link})
			if len(out) >= 10000 {
				return out
			}
		}
	}
	return out
}
func compatibilityPayload(nodes []models.Node, config string) gin.H {
	inputs := compatibilityInputs(nodes)
	reports := node.BuildCompatibility(inputs)
	var cfg node.SqlConfig
	_ = json.Unmarshal([]byte(config), &cfg)
	if len(cfg.AppRules) > 0 {
		for i := range reports {
			if reports[i].Key == "clash" || reports[i].Key == "singbox" {
				continue
			}
			reports[i].Included, reports[i].Warnings, reports[i].Excluded = 0, 0, reports[i].Total
			for j := range reports[i].Items {
				reports[i].Items[j].Status = "unsupported"
				reports[i].Items[j].Reason = "此订阅包含应用规则，仅支持 Mihomo、sing-box 完整配置"
			}
		}
	}
	return gin.H{"limited": len(inputs) >= 10000, "revision": node.CompatibilityRevision, "clients": reports, "note": "检查当前导出器能否保留节点参数，不代表网络连通性；客户端版本要求见提示"}
}

// SubscriptionCompatibility uses the same groups, delivery policy and pipeline as export.
func SubscriptionCompatibility(c *gin.Context) {
	id, e := strconv.Atoi(c.Query("id"))
	if e != nil || id < 1 {
		c.JSON(400, gin.H{"code": "40000", "msg": "订阅编号无效"})
		return
	}
	var sub models.Subcription
	if models.DB.First(&sub, id).Error != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "订阅不存在"})
		return
	}
	if e := mergeGroupNodes(&sub); e != nil {
		c.JSON(502, gin.H{"code": "50200", "msg": "订阅节点加载失败，请稍后重试"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": compatibilityPayload(sub.Nodes, sub.Config)})
}
func NodeCompatibility(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	var req struct {
		IDs []int `json:"ids"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.IDs) > 2000 {
		c.JSON(400, gin.H{"code": "40000", "msg": "一次最多检查 2000 个节点"})
		return
	}
	var nodes []models.Node
	query := models.DB.Order("id asc").Limit(2001)
	if len(req.IDs) > 0 {
		query = query.Where("id IN ?", req.IDs)
	}
	if query.Find(&nodes).Error != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点失败"})
		return
	}
	if len(nodes) > 2000 {
		c.JSON(400, gin.H{"code": "40000", "msg": "节点较多，请分批选择后检查"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": compatibilityPayload(models.FilterVisibleNodes(nodes), "")})
}
func clientFormat(client string) string {
	switch client {
	case "stash", "egern":
		return "clash"
	case "surfboard":
		return "surge"
	default:
		return client
	}
}
func filterExportInputs(c *gin.Context, urls []string, fallback string) ([]string, bool) {
	client := c.GetString("compatibilityClient")
	if client == "" {
		client = fallback
	}
	links, _ := node.FilterClientLinks(urls, client)
	if len(links) == 0 {
		c.String(422, node.ErrNoCompatibleNodes.Error())
		return nil, false
	}
	return links, true
}
func writeClientExportError(c *gin.Context, err error) {
	if errors.Is(err, node.ErrNoCompatibleNodes) {
		c.String(422, err.Error())
	} else {
		c.String(500, "客户端配置生成失败，请检查模板和节点参数")
	}
}
