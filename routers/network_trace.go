package routers

import (
	"github.com/gin-gonic/gin"
	"ppeelink/api"
)

func NetworkTraces(r *gin.Engine) {
	g := r.Group("/api/v1/network/traces")
	g.GET("/sources", api.NetworkTraceSources)
	g.GET("/list", api.NetworkTraceList)
	g.GET("/detail", api.NetworkTraceDetail)
	g.POST("/create", api.NetworkTraceCreate)
	g.POST("/cancel", api.NetworkTraceCancel)
	r.GET("/api/v1/probe/trace/jobs", api.ProbeTraceJobs)
	r.POST("/api/v1/probe/trace/report", api.ProbeTraceReport)
	r.GET("/api/v1/agent/trace/jobs", api.AgentTraceJobs)
	r.POST("/api/v1/agent/trace/report", api.AgentTraceReport)
}
