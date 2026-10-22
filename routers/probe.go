package routers

import (
	"ppeelink/api"

	"github.com/gin-gonic/gin"
)

func Probes(r *gin.Engine) {
	// 探针端接口：使用 Probe <token> 认证，不走 JWT。
	r.GET("/api/v1/probe/jobs", api.ProbeJobs)
	r.POST("/api/v1/probe/report", api.ProbeReport)
	r.GET("/api/v1/probe/binary", api.ProbeBinary)

	// 管理端接口：JWT。
	g := r.Group("/api/v1/probes")
	{
		g.GET("/list", api.ProbeList)
		g.POST("/add", api.ProbeAdd)
		g.POST("/delete", api.ProbeDelete)
		g.POST("/reset-token", api.ProbeResetToken)
	}
}
