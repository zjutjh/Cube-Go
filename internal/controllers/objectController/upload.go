package objectController

import (
	"errors"
	"image"
	"io"
	"mime/multipart"
	"path/filepath"

	"cube-go/internal/apiException"
	"cube-go/internal/services/objectService"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
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
	var data uploadFileData
	if err := c.ShouldBind(&data); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	bucket, err := oss.Buckets.GetBucket(data.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}

	if data.File.Size > objectService.SizeLimit {
		apiException.AbortWithException(c, apiException.FileSizeExceedError, nil)
		return
	}

	filename := data.File.Filename
	ext := filepath.Ext(filename)             // 获取文件扩展名
	name := filename[:len(filename)-len(ext)] // 获取去掉扩展名的文件名

	// 若使用 UUID 作为文件名
	if data.UseUUID {
		name = uuid.NewV1().String()
	}

	file, err := data.File.Open()
	if err != nil {
		apiException.AbortWithException(c, apiException.UploadFileError, err)
		return
	}
	defer func() { _ = file.Close() }()

	// 转换到 WebP
	var reader io.ReadSeeker = file
	if data.ConvertWebP {
		reader, err = objectService.ConvertToWebP(file)
		ext = ".webp"
		if errors.Is(err, image.ErrFormat) {
			apiException.AbortWithException(c, apiException.FileNotImageError, err)
			return
		}
		if err != nil {
			apiException.AbortWithException(c, apiException.ServerError, err)
			return
		}
	}

	// 上传文件
	objectKey := objectService.GenerateObjectKey(data.Location, name, ext)
	err = bucket.SaveObject(reader, objectKey)
	if errors.Is(err, oss.ErrFileAlreadyExists) {
		apiException.AbortWithException(c, apiException.FileAlreadyExists, err)
		return
	}
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	zap.L().Info("上传文件成功", zap.String("bucket", data.Bucket), zap.String("objectKey", objectKey), zap.String("ip", c.ClientIP()))
	response.JsonSuccessResp(c, gin.H{
		"object_key": objectKey,
	})
}
