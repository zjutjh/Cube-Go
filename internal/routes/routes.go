package routes

import (
	"cube-go/internal/controllers/objectController"
	"cube-go/internal/midwares"
	"github.com/gin-gonic/gin"
)

// Init 初始化路由
func Init(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/buckets", midwares.Auth, objectController.GetBucketList)
		api.POST("/upload", midwares.Auth, objectController.UploadFile)
		api.GET("/files", midwares.Auth, objectController.GetFileList)
		api.DELETE("/delete", midwares.Auth, objectController.DeleteFile)

		api.GET("/file", objectController.GetFile)
	}
}
