package routes

import (
	"github.com/gin-gonic/gin"
)

// Init 初始化路由
func Init(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.POST("")
	}
}
