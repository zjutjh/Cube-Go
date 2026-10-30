package objectController

import (
	"errors"
	"mime/multipart"
	"net/http"

	"cube-go/internal/apiException"
	"cube-go/internal/services/objectService"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type uploadFileData struct {
	File        *multipart.FileHeader `form:"file" binding:"required"`
	Bucket      string                `form:"bucket" binding:"required"`
	Location    string                `form:"location"`
	ConvertWebP bool                  `form:"convert_webp"`
	UseUUID     bool                  `form:"use_uuid"`
}

// UploadFile 上传文件
func UploadFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, objectService.SizeLimit)

	var data uploadFileData
	if err := c.ShouldBind(&data); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			apiException.AbortWithException(c, apiException.FileSizeExceedError, err)
			return
		}
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	bucket, err := oss.Buckets.GetBucket(data.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}

	objectKey, _, err := saveUploaded(c, bucket, data.File, data.Location, data.UseUUID, data.ConvertWebP)
	if err != nil {
		abortUploadErr(c, err)
		return
	}

	zap.L().Info("上传文件成功", zap.String("bucket", data.Bucket), zap.String("objectKey", objectKey), zap.String("ip", c.ClientIP()))
	response.JsonSuccessResp(c, gin.H{
		"object_key": objectKey,
	})
}
