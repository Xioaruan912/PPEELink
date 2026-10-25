package routers

import (
	"ppeelink/api"

	"github.com/gin-gonic/gin"
)

// Proxy 自建节点：服务器、入站、账户配额（管理员），agent 同步接口与个人订阅（令牌认证）。
func Proxy(r *gin.Engine) {
	g := r.Group("/api/v1/proxy")
	{
		g.GET("/egress-options", api.ProxyEgressOptions)
		g.GET("/servers", api.ProxyServerList)
		g.POST("/servers/save", api.ProxyServerSave)
		g.POST("/servers/reset-token", api.ProxyServerResetToken)
		g.POST("/servers/update-agent", api.ProxyServerUpdateAgent)
		g.DELETE("/servers/delete", api.ProxyServerDelete)
		g.POST("/inbounds/save", api.ProxyInboundSave)
		g.DELETE("/inbounds/delete", api.ProxyInboundDelete)
		g.POST("/check-target", api.ProxyCheckTarget)
		g.GET("/accounts", api.ProxyAccountList)
		g.POST("/accounts/save", api.ProxyAccountSave)
		g.GET("/traffic", api.ProxyTrafficDaily)
		g.GET("/website-traffic", api.ProxyWebsiteTraffic)
	}
	r.GET("/api/v1/my/self-hosted", api.MySelfHosted)
	r.GET("/api/v1/agent/spec", api.AgentSpec)
	r.POST("/api/v1/agent/report", api.AgentReport)
	r.GET("/api/v1/agent/ws", api.AgentWebSocket)
	r.GET("/api/v1/agent/checks/jobs", api.AgentCheckJobs)
	r.POST("/api/v1/agent/checks/report", api.AgentCheckReport)
	r.GET("/agent/install.sh", api.AgentInstallScript)
	r.GET("/agent/binary", api.AgentBinary)
	r.GET("/p/", api.GetPersonalSubscription)
}
