package routers

import (
	"ppeelink/api"
	"ppeelink/middlewares"

	"github.com/gin-gonic/gin"
)

func Clients(r *gin.Engine) {
	ClientsGroup := r.Group("/c")
	ClientsGroup.Use(middlewares.GetIp)
	{
		ClientsGroup.GET("/", api.GetClient)
	}
	// 单节点临时订阅（签名链接，到期自动失效）
	r.GET("/t/", api.GetTempLink)

}
