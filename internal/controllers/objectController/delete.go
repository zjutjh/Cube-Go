package objectController

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"jh-oss/internal/apiException"
	"jh-oss/internal/services/objectService"
	"jh-oss/pkg/response"
)

type deleteFileData struct {
	Target string `json:"target" binding:"required"`
}

// DeleteFile 删除文件或目录
func DeleteFile(c *gin.Context) {
	var data deleteFileData
	if err := c.ShouldBindJSON(&data); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	target := objectService.CleanLocation(data.Target)
	if target == "" { // 拦截删除根目录的请求
		apiException.AbortWithException(c, apiException.ParamError, nil)
		return
	}

	filePath := filepath.Join("static", target)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		apiException.AbortWithException(c, apiException.LocationNotFound, err)
		return
	}

	err := os.RemoveAll(filePath) // 使用 RemoveAll 删除文件或目录
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	response.JsonSuccessResp(c, nil)
}
