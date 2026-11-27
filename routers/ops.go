package routers

import (
	"github.com/gin-gonic/gin"
	"ppeelink/api"
)

func Ops(r *gin.Engine) {
	g := r.Group("/api/v1/ops")
	g.GET("/backup/export", api.ConfigBackupExport)
	g.GET("/backup/full", api.ConfigBackupFull)
	g.POST("/backup/full/restore", api.ConfigBackupFullRestore)
	g.GET("/backup/inspect", api.ConfigBackupInspect)
	g.POST("/backup/import", api.ConfigBackupImport)
	g.GET("/backup/webdav/config", api.WebDAVBackupGetConfig)
	g.PUT("/backup/webdav/config", api.WebDAVBackupSaveConfig)
	g.POST("/backup/webdav/test", api.WebDAVBackupTest)
	g.POST("/backup/webdav/run", api.WebDAVBackupRunNow)
	g.GET("/backup/webdav/list", api.WebDAVBackupList)
	g.POST("/backup/webdav/restore", api.WebDAVBackupRestore)
}
