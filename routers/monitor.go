package routers

import (
	"ppeelink/api"

	"github.com/gin-gonic/gin"
)

func Monitors(r *gin.Engine) {
	// 监控端接口：使用 Monitor <token> 认证，不走 JWT。
	r.GET("/api/v1/monitor/config", api.MonitorConfig)
	r.POST("/api/v1/monitor/report", api.MonitorReport)
	r.GET("/api/v1/monitor/binary", api.MonitorBinary)
	r.GET("/api/v1/monitor/install.sh", api.MonitorInstallScript)
	r.GET("/api/v1/monitors/ssh/ws", api.MonitorAgentTerminal)

	// 管理端接口：JWT。
	g := r.Group("/api/v1/monitors")
	{
		g.GET("/list", api.MonitorList)
		g.GET("/detail", api.MonitorDetail)
		g.POST("/add", api.MonitorAdd)
		g.POST("/delete", api.MonitorDelete)
		g.POST("/reset-token", api.MonitorResetToken)
		g.POST("/ssh/ticket", api.MonitorSSHTicket)
	}
}
