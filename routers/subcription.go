package routers

import (
	"ppeelink/api"

	"github.com/gin-gonic/gin"
)

func Subcription(r *gin.Engine) {
	SubcriptionGroup := r.Group("/api/v1/subcription")
	{
		SubcriptionGroup.POST("/add", api.SubAdd)
		SubcriptionGroup.DELETE("/delete", api.SubDel)
		SubcriptionGroup.GET("/get", api.SubGet)
		SubcriptionGroup.GET("/preview-nodes", api.SubPreviewNodes)
		SubcriptionGroup.GET("/compatibility", api.SubscriptionCompatibility)
		SubcriptionGroup.POST("/pipeline/preview", api.SubPipelinePreview)
		SubcriptionGroup.POST("/update", api.SubUpdate)
		SubcriptionGroup.POST("/reset-token", api.ResetSubToken)
		SubcriptionGroup.POST("/set-expire", api.SetSubExpire)
		SubcriptionGroup.POST("/check-alerts", api.SubscriptionCheckAlerts)
		SubcriptionGroup.GET("/usage", api.SubscriptionUsage)
	}

}
