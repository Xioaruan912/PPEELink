package routers

import (
	"ppeelink/api"

	"github.com/gin-gonic/gin"
)

// MonitorOperations registers admin-only remote file and container operations.
// Call after the application's authentication middleware has been installed.
func MonitorOperations(r *gin.Engine) {
	g := r.Group("/api/v1/monitors", api.MonitorOperationsAdmin)
	g.GET("/files/list", api.MonitorFilesList)
	g.GET("/files/read", api.MonitorFilesRead)
	g.GET("/files/download", api.MonitorFilesRead)
	g.POST("/files/save", api.MonitorFilesWrite)
	g.POST("/files/upload", api.MonitorFilesWrite)
	g.POST("/files/mkdir", api.MonitorFilesMutate)
	g.POST("/files/rename", api.MonitorFilesMutate)
	g.POST("/files/delete", api.MonitorFilesMutate)
	g.GET("/docker/:operation", api.MonitorDocker)
	g.POST("/docker/:operation", api.MonitorDocker)
}
