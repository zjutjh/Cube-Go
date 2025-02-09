package routes

import (
	"github.com/gin-gonic/gin"
	"jh-oss/internal/controllers/objectController"
	"jh-oss/internal/midwares"
)

// Init 初始化路由
func Init(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.POST("/upload", objectController.UploadFile)
		api.GET("/files", midwares.Auth, objectController.GetFileList)
		api.DELETE("/delete", midwares.Auth, objectController.DeleteFile)
	}
}
