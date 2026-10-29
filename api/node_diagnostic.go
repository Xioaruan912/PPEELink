package api

import (
	"context"
	"encoding/json"
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
	if req.ID > 0 {
		var n models.Node
		if models.DB.First(&n, req.ID).Error != nil {
			c.JSON(404, gin.H{"code": "40400", "msg": "节点不存在"})
			return
		}
		if n.Flags != "" && n.Flags != "{}" {
			var flags node.NodeFlags
			if json.Unmarshal([]byte(n.Flags), &flags) != nil || flags.DialerProxyID != 0 || flags.SkipCertVerify {
				c.JSON(400, gin.H{"code": "40000", "msg": "该节点含链式代理或跳过证书设置，请使用完整配置验证；一键诊断不会忽略这些设置"})
				return
			}
		}
		req.Link = n.Link
	}
	if strings.TrimSpace(req.Link) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "请选择节点或填写链接"})
		return
	}
	if req.Target == "" {
		req.Target = "cloudflare"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	result, err := diagnostic.Run(ctx, req.Link, req.Target)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": result, "msg": "诊断完成"})
}
