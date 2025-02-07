package routes

import (
	"github.com/gin-gonic/gin"
	"jh-oss/internal/controllers/objectController"
)

// Init 初始化路由
func Init(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.POST("/upload", objectController.UploadFile)
	}
}
