package routers

import (
	"ppeelink/api"

	"github.com/gin-gonic/gin"
)

func User(r *gin.Engine) {
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/login", api.UserLogin)
		authGroup.DELETE("/logout", api.UserOut)
	}
	userGroup := r.Group("/api/v1/users")
	{
		userGroup.GET("/me", api.UserMe)
		userGroup.POST("/update", api.UserSet)
		// 以下为管理员接口（中间件限制普通用户访问）
		userGroup.GET("/page", api.UserPages)
		userGroup.POST("/create", api.UserCreate)
		userGroup.POST("/edit", api.UserEdit)
		userGroup.DELETE("/delete", api.UserDelete)
	}
	r.GET("/api/v1/my/subscriptions", api.MySubscriptions)
}
