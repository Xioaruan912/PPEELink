package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"ppeelink/models"
)

// MonitorUpdateAgent also routes linked monitors to their existing agent. Proxy
// Enabled is deliberately not checked: stopping xray must not disable management.
func MonitorUpdateAgent(c *gin.Context) {
	var req struct {
		ID     uint `json:"id"`
		Cancel bool `json:"cancel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "40000", "msg": "服务器 id 无效"})
		return
	}
	var monitor models.Monitor
	if models.DB.First(&monitor, req.ID).Error != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
		return
	}
	requested := !req.Cancel
	var err error
	if monitor.ProxyServerID != nil {
		result := models.DB.Model(&models.ProxyServer{}).Where("id = ? AND kind = ?", *monitor.ProxyServerID, models.ServerKindAgent).Update("agent_update_requested", requested)
		err = result.Error
		if err == nil && result.RowsAffected == 0 {
			c.JSON(400, gin.H{"code": "40000", "msg": "本机服务器无需升级 agent"})
			return
		}
	} else {
		err = models.DB.Model(&monitor).Updates(map[string]any{"agent_update_requested": requested, "agent_update_error": ""}).Error
	}
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "更新请求保存失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"requested": requested}, "msg": "升级请求已保存，等待客户端同步"})
}
