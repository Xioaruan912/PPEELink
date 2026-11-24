package routers

import (
	"ppeelink/api"

	"github.com/gin-gonic/gin"
)

func Probes(r *gin.Engine) {
	// 用户 Agent 端接口：使用 Probe <token> 认证，不走 JWT。
	r.GET("/api/v1/probe/wait", api.ProbeWait)
	r.GET("/api/v1/probe/jobs", api.ProbeJobs)
	r.POST("/api/v1/probe/report", api.ProbeReport)
	r.POST("/api/v1/probe/trace-report", api.UserProbeTraceReport)
	r.POST("/api/v1/probe/unlock-report", api.UserProbeUnlockReport)
	r.POST("/api/v1/probe/speedtest-report", api.UserProbeSpeedtestReport)
	r.POST("/api/v1/probe/clash-report", api.UserProbeClashReport)
	r.POST("/api/v1/probe/ack", api.ProbeAck)
	r.GET("/api/v1/probe/download", api.ProbeDownloadAuth)
	r.GET("/api/v1/probe/binary", api.ProbeBinary)

	// 管理端接口：JWT。
	g := r.Group("/api/v1/probes")
	{
		g.GET("/list", api.ProbeList)
		g.POST("/add", api.ProbeAdd)
		g.POST("/delete", api.ProbeDelete)
		g.POST("/reset-token", api.ProbeResetToken)
		g.POST("/trigger", api.ProbeTrigger)
		g.POST("/update", api.ProbeUpdate)
		g.POST("/auto-toggle", api.ProbeAutoToggle)
		g.POST("/trace", api.ProbeTrace)
		g.POST("/trace-bidirectional", api.ProbeBidirectionalTrace)
		g.POST("/unlock", api.ProbeUnlock)
		g.POST("/speedtest", api.ProbeSpeedtest)
		g.POST("/diagnose-local", api.ProbeDiagnoseLocal)
		g.POST("/clash/status", api.ProbeClashStatus)
		g.POST("/clash/switch", api.ProbeClashSwitch)
		g.POST("/clash/batch-test", api.ProbeClashBatchTest)
		g.POST("/push-config", api.ProbePushConfig)
		g.GET("/topology", api.ProbeTopology)
		g.POST("/apply-sort", api.ProbeApplySort)
		g.GET("/sparklines", api.ProbeSparklines)
		g.GET("/history", api.ProbeHistory)
		g.GET("/download", api.ProbeDownload)
	}
}
