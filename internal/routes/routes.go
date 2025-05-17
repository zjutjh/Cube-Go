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
		api.POST("/upload", midwares.Auth, objectController.BatchUploadFiles)
		api.GET("/files", midwares.Auth, objectController.GetFileList)
		api.DELETE("/delete", midwares.Auth, objectController.DeleteFile)
		api.GET("/file", objectController.GetFile)
		// api.POST("/create-dir", objectController.CreateDir)
	}
}
