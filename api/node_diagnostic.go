package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"ppeelink/diagnostic"
	"ppeelink/models"
	"ppeelink/node"
	"strings"
	"time"
)

func NodeDiagnostic(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20000)
	var req struct {
		ID     int    `json:"id"`
		Link   string `json:"link"`
		Target string `json:"target"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "诊断参数错误"})
		return
	}
	var path []diagnostic.Hop
	if req.ID > 0 {
		var err error
		path, err = diagnosticPath(req.ID)
		if err != nil {
			c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
			return
		}
	} else {
		path = []diagnostic.Hop{{Name: "粘贴的节点", Link: req.Link}}
	}
	if req.ID <= 0 && strings.TrimSpace(req.Link) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "请选择节点或填写链接"})
		return
	}
	if req.Target == "" {
		req.Target = "cloudflare"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	result, err := diagnostic.RunPath(ctx, path, req.Target)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": result, "msg": "诊断完成"})
}

// diagnosticPath uses the saved entry endpoint directly. Server-side gateways
// and native relays are traversed by the real connection, not client policies.
func diagnosticPath(id int) ([]diagnostic.Hop, error) {
	var n models.Node
	if models.DB.First(&n, id).Error != nil {
		return nil, fmt.Errorf("节点已删除或不存在（ID %d）", id)
	}
	var flags node.NodeFlags
	if n.Flags != "" && json.Unmarshal([]byte(n.Flags), &flags) != nil {
		return nil, fmt.Errorf("%s 的连接设置无效", n.Name)
	}
	if flags.SkipCertVerify {
		return nil, fmt.Errorf("%s 开启了跳过证书校验，请先使用可信证书或证书指纹", n.Name)
	}
	return []diagnostic.Hop{{Name: n.Name, Link: n.Link}}, nil
}
