// api/subcription.go

package api

import (
	"fmt"

	"strconv"
	"strings"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

// parseGroups 解析逗号分隔的分组 ID 参数为 GroupNode 引用列表
func parseGroups(groups string) ([]models.GroupNode, error) {
	var refs []models.GroupNode
	if groups == "" {
		return refs, nil
	}
	for _, g := range strings.Split(groups, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(g))
		if err != nil || id == 0 {
			continue
		}
		var gn models.GroupNode
		if err := models.DB.Where("id = ?", id).First(&gn).Error; err != nil {
			return nil, err
		}
		if gn.Hidden {
			return nil, fmt.Errorf("分组 %s 已全局隐藏，不能用于订阅", gn.Name)
		}
		refs = append(refs, models.GroupNode{ID: gn.ID})
	}
	return refs, nil
}

func SubTotal(c *gin.Context) {
	var Sub models.Subcription
	subs, err := Sub.List()
	count := len(subs)
	if err != nil {
		c.JSON(500, gin.H{
			"msg": "取得订阅总数失败",
		})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": count,
		"msg":  "取得订阅总数",
	})
}

// 获取订阅列表
func SubGet(c *gin.Context) {
	var Sub models.Subcription
	Subs, err := Sub.List()
	if err != nil {
		c.JSON(500, gin.H{
			"msg": "node list error",
		})
		return
	}
	hiddenIDs, _ := models.GloballyHiddenNodeIDs()
	// 加载每个订阅引用的分组（含可见节点数），供前端回显
	for i := range Subs {
		Subs[i].Nodes = models.FilterVisibleNodes(Subs[i].Nodes)
		models.DB.Preload("GroupRefs").First(&Subs[i], Subs[i].ID)
		visibleRefs := Subs[i].GroupRefs[:0]
		for j := range Subs[i].GroupRefs {
			if Subs[i].GroupRefs[j].Hidden {
				continue
			}
			var ids []int
			models.DB.Table("group_node_nodes AS x").Select("x.node_id").
				Joins("JOIN nodes AS n ON n.id = x.node_id AND n.deleted_at IS NULL AND n.hidden = ?", false).
				Where("x.group_node_id = ?", Subs[i].GroupRefs[j].ID).Scan(&ids)
			count := 0
			for _, id := range ids {
				if !hiddenIDs[id] {
					count++
				}
			}
			Subs[i].GroupRefs[j].NodeCount = count
			visibleRefs = append(visibleRefs, Subs[i].GroupRefs[j])
		}
		Subs[i].GroupRefs = visibleRefs
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": Subs,
		"msg":  "node get",
	})
}

// SubPreviewNodes 预览合并后的订阅节点（不记录日志、不下发配置）
func SubPreviewNodes(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "id 不能为空"})
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "无效的 ID"})
		return
	}
	var sub models.Subcription
	if err := models.DB.First(&sub, id).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "订阅不存在"})
		return
	}
	excluded, err := mergeGroupNodesDetailed(&sub)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "合并节点失败", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code":     "00000",
		"data":     sub.Nodes,
		"excluded": excluded,
		"msg":      "节点预览",
	})
}

// 添加订阅
func SubAdd(c *gin.Context) {
	name := c.PostForm("name")
	configs := c.PostForm("config")
	nodes := c.PostForm("nodes")
	groups := c.PostForm("groups")
	pipeline := c.PostForm("pipeline")
	if err := ensureScriptAllowed(c, pipeline); err != nil {
		c.JSON(403, gin.H{"code": 403, "msg": err.Error()})
		return
	}

	if name == "" {
		c.JSON(400, gin.H{
			"msg": "订阅名称不能为空",
		})
		return
	}
	if nodes == "" && groups == "" {
		c.JSON(400, gin.H{
			"msg": "必须选择节点或分组",
		})
		return
	}

	groupRefs, err := parseGroups(groups)
	if err != nil {
		c.JSON(400, gin.H{"msg": "分组引用无效: " + err.Error()})
		return
	}

	var NodesData []models.Node
	var nodeNames []string
	for _, nodeName := range strings.Split(nodes, ",") {
		if strings.TrimSpace(nodeName) == "" {
			continue
		}
		FirstNode := models.Node{
			Name: nodeName,
		}
		result := models.DB.Model(models.Node{}).Where("name = ?", FirstNode.Name).First(&FirstNode)
		if result.Error != nil {
			c.JSON(400, gin.H{"msg": "节点不存在: " + result.Error.Error()})
			return
		}
		NodesData = append(NodesData, FirstNode)
		nodeNames = append(nodeNames, FirstNode.Name)
	}

	sub := models.Subcription{
		Name:             name,
		Config:           configs,
		NodeOrder:        strings.Join(nodeNames, ","),
		Nodes:            NodesData,
		GroupRefs:        groupRefs,
		Pipeline:         pipeline,
		LastGoodSnapshot: strings.Join(nodeNames, "\n"),
	}
	err = sub.Add()
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "添加订阅失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  "添加订阅成功",
	})
}

// 更新订阅
func SubUpdate(c *gin.Context) {
	NewName := c.PostForm("name")
	OldName := c.PostForm("oldname")
	configs := c.PostForm("config")
	nodes := c.PostForm("nodes")
	groups := c.PostForm("groups")
	pipeline := c.PostForm("pipeline")
	if err := ensureScriptAllowed(c, pipeline); err != nil {
		c.JSON(403, gin.H{"code": 403, "msg": err.Error()})
		return
	}

	if NewName == "" {
		c.JSON(400, gin.H{
			"msg": "订阅名称不能为空",
		})
		return
	}
	if nodes == "" && groups == "" {
		c.JSON(400, gin.H{
			"msg": "必须选择节点或分组",
		})
		return
	}

	groupRefs, err := parseGroups(groups)
	if err != nil {
		c.JSON(400, gin.H{"msg": "分组引用无效: " + err.Error()})
		return
	}

	var NodesData []models.Node
	var nodeNames []string
	for _, nodeName := range strings.Split(nodes, ",") {
		if strings.TrimSpace(nodeName) == "" {
			continue
		}
		FirstNode := models.Node{
			Name: nodeName,
		}
		result := models.DB.Model(models.Node{}).Where("name = ?", FirstNode.Name).First(&FirstNode)
		if result.Error != nil {
			c.JSON(400, gin.H{"msg": result.Error.Error()})
			return
		}
		NodesData = append(NodesData, FirstNode)
		nodeNames = append(nodeNames, FirstNode.Name)
	}

	OldSub := models.Subcription{Name: OldName}
	NewSub := models.Subcription{
		Name:             NewName,
		Config:           configs,
		NodeOrder:        strings.Join(nodeNames, ","),
		Nodes:            NodesData,
		GroupRefs:        groupRefs,
		Pipeline:         pipeline,
		LastGoodSnapshot: strings.Join(nodeNames, "\n"),
	}
	err = OldSub.Update(&NewSub)
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "更新订阅失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  "更新订阅成功",
	})
}

// 删除订阅 (无需修改)
func SubDel(c *gin.Context) {
	var sub models.Subcription
	id := c.Query("id")
	if id == "" {
		c.JSON(400, gin.H{
			"msg": "id 不能为空",
		})
		return
	}
	x, err := strconv.Atoi(id) // 增加错误检查
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "无效的 ID: " + err.Error(),
		})
		return
	}
	sub.ID = x
	err = sub.Find()
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "查找订阅失败: " + err.Error(),
		})
		return
	}
	err = sub.Del()
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "删除订阅失败: " + err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  "删除订阅成功",
	})
}

// 重置订阅令牌（重置后旧订阅链接立即失效）
// POST /api/v1/subcription/reset-token  body: id
func ResetSubToken(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "id 不能为空"})
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "无效的 ID"})
		return
	}
	sub := models.Subcription{ID: id}
	if err := sub.Find(); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "查找订阅失败: " + err.Error()})
		return
	}
	newToken := models.GenerateToken()
	if err := models.DB.Model(&sub).Update("token", newToken).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "重置令牌失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"token": newToken}, "msg": "订阅链接已重置"})
}

// 设置/清除订阅过期时间
// POST /api/v1/subcription/set-expire  body: id + expire(Unix秒, 留空=永不过期)
func SetSubExpire(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "id 不能为空"})
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "无效的 ID"})
		return
	}
	sub := models.Subcription{ID: id}
	if err := sub.Find(); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "查找订阅失败: " + err.Error()})
		return
	}
	expireStr := c.PostForm("expire")
	var expiresAt *time.Time
	if expireStr != "" {
		ts, err := strconv.ParseInt(expireStr, 10, 64)
		if err != nil {
			c.JSON(400, gin.H{"code": "40000", "msg": "过期时间格式错误"})
			return
		}
		t := time.Unix(ts, 0)
		expiresAt = &t
	}
	if err := models.DB.Model(&sub).Update("expires_at", expiresAt).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "设置过期失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "过期时间已更新"})
}
