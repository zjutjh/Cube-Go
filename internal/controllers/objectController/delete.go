package objectController

import (
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/apiException"
	"jh-oss/internal/services/objectService"
	"jh-oss/pkg/oss"
	"jh-oss/pkg/response"
)

type deleteFileData struct {
	Bucket    string `form:"bucket" binding:"required"`
	ObjectKey string `form:"object_key" binding:"required"`
}

// DeleteFile 删除文件或目录
func DeleteFile(c *gin.Context) {
	var data deleteFileData
	if err := c.ShouldBind(&data); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	bucket, err := oss.Buckets.GetBucket(data.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}

	target := objectService.CleanLocation(data.ObjectKey)
	if target == "" { // 拦截删除根目录的请求
		apiException.AbortWithException(c, apiException.ParamError, nil)
		return
	}

	err = bucket.DeleteObject(target)
	if errors.Is(err, oss.ErrResourceNotExists) {
		apiException.AbortWithException(c, apiException.ResourceNotFound, err)
		return
	}
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	zap.L().Info("删除文件成功", zap.String("bucket", data.Bucket), zap.String("target", target), zap.String("ip", c.ClientIP()))
	response.JsonSuccessResp(c, nil)
}
