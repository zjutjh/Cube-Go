package objectController

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/apiException"
	"jh-oss/internal/services/objectService"
	"jh-oss/pkg/config"
	"jh-oss/pkg/response"
)

type createDirData struct {
	Target string `json:"target" binding:"required"`
}

// CreateDir 创建文件夹
func CreateDir(c *gin.Context) {
	var data createDirData
	if err := c.ShouldBindJSON(&data); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	target := objectService.CleanLocation(data.Target)
	filePath := filepath.Join(config.OSSFolder, target)
	if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	zap.L().Info("创建文件夹成功", zap.String("target", target), zap.String("ip", c.ClientIP()))
	response.JsonSuccessResp(c, nil)
}
