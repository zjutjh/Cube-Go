package objectController

import (
	"errors"
	"image"
	"io"
	"mime/multipart"
	"path/filepath"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
	"jh-oss/internal/apiException"
	"jh-oss/internal/services/objectService"
	"jh-oss/pkg/oss"
	"jh-oss/pkg/response"
)

type batchUploadFileData struct {
	Files       []*multipart.FileHeader `form:"files" binding:"required"`
	Bucket      string                  `form:"bucket" binding:"required"`
	Location    string                  `form:"location"`
	DontConvert bool                    `form:"dont_convert"`
	RetainName  bool                    `form:"retain_name"`
}

type uploadFileRespElement struct {
	Filename  string `json:"filename"`
	ObjectKey string `json:"object_key,omitempty"`
	Error     string `json:"error,omitempty"`
}

// BatchUploadFiles 批量上传文件
func BatchUploadFiles(c *gin.Context) {
	var data batchUploadFileData
	if err := c.ShouldBind(&data); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	bucket, err := oss.Buckets.GetBucket(data.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}

	results := make([]uploadFileRespElement, 0)
	for _, fileHeader := range data.Files {
		element := uploadFileRespElement{
			Filename: fileHeader.Filename,
		}

		fileSize := fileHeader.Size
		if fileSize > objectService.SizeLimit {
			element.Error = apiException.FileSizeExceedError.Error()
			results = append(results, element)
			continue
		}

		u := uuid.NewV1().String()
		filename := fileHeader.Filename
		ext := filepath.Ext(filename)             // 获取文件扩展名
		name := filename[:len(filename)-len(ext)] // 获取去掉扩展名的文件名

		// 若不保留文件名，则使用 UUID 作为文件名
		if !data.RetainName {
			name = u
		}

		file, err := fileHeader.Open()
		if err != nil {
			element.Error = apiException.UploadFileError.Error()
			results = append(results, element)
			continue
		}

		// 转换到 WebP
		var reader io.Reader = file
		if !data.DontConvert {
			reader, err = objectService.ConvertToWebP(file)
			ext = ".webp"
			if errors.Is(err, image.ErrFormat) {
				element.Error = apiException.FileNotImageError.Error()
				results = append(results, element)
				continue
			}
			if err != nil {
				element.Error = apiException.ServerError.Error()
				results = append(results, element)
				continue
			}
		}

		// 上传文件
		objectKey := objectService.GenerateObjectKey(data.Location, name, ext)
		err = bucket.SaveObject(reader, objectKey)
		if err != nil {
			element.Error = apiException.ServerError.Error()
			results = append(results, element)
			continue
		}

		element.ObjectKey = objectKey
		results = append(results, element)

		zap.L().Info("上传文件成功", zap.String("bucket", data.Bucket), zap.String("objectKey", objectKey), zap.String("ip", c.ClientIP()))

		// 关闭文件
		_ = file.Close()
	}

	response.JsonSuccessResp(c, gin.H{
		"results": results,
	})
}
