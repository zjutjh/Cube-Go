package routes

import (
	"github.com/gin-gonic/gin"
	"jh-oss/internal/controllers/objectController"
	"jh-oss/internal/midwares"
)

// Init 初始化路由
func Init(r *gin.Engine) {
	api := r.Group("/api", midwares.Auth)
	{
		api.POST("/upload", objectController.BatchUploadFiles)
		api.GET("/files", objectController.GetFileList)
		api.DELETE("/delete", objectController.DeleteFile)
		// api.POST("/create-dir", objectController.CreateDir)
	}
}
