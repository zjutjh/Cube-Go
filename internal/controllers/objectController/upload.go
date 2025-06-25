package objectController

import (
	"errors"
	"image"
	"io"
	"mime/multipart"
	"path/filepath"
	"sync"

	"cube-go/internal/apiException"
	"cube-go/internal/services/objectService"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
)

type batchUploadFileData struct {
	Files       []*multipart.FileHeader `form:"files" binding:"required"`
	Bucket      string                  `form:"bucket" binding:"required"`
	Location    string                  `form:"location"`
	ConvertWebP bool                    `form:"convert_webp"`
	UseUUID     bool                    `form:"use_uuid"`
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

	var mutex sync.Mutex
	var wg sync.WaitGroup
	results := make([]uploadFileRespElement, 0, len(data.Files))
	for _, f := range data.Files {
		wg.Add(1)
		go func(fileHeader *multipart.FileHeader) {
			defer wg.Done()

			res := handleSingleUpload(fileHeader, &data, bucket, c.ClientIP())
			mutex.Lock()
			results = append(results, res)
			mutex.Unlock()
		}(f)
	}

	wg.Wait()
	response.JsonSuccessResp(c, gin.H{
		"results": results,
	})
}

func handleSingleUpload(
	fileHeader *multipart.FileHeader, data *batchUploadFileData, bucket oss.StorageProvider, ip string,
) uploadFileRespElement {
	element := uploadFileRespElement{
		Filename: fileHeader.Filename,
	}

	if fileHeader.Size > objectService.SizeLimit {
		element.Error = apiException.FileSizeExceedError.Error()
		return element
	}

	filename := fileHeader.Filename
	ext := filepath.Ext(filename)             // 获取文件扩展名
	name := filename[:len(filename)-len(ext)] // 获取去掉扩展名的文件名

	// 若使用 UUID 作为文件名
	if data.UseUUID {
		name = uuid.NewV1().String()
	}

	file, err := fileHeader.Open()
	if err != nil {
		element.Error = apiException.UploadFileError.Error()
		return element
	}
	defer func() { _ = file.Close() }()

	// 转换到 WebP
	var reader io.ReadSeeker = file
	if data.ConvertWebP {
		reader, err = objectService.ConvertToWebP(file)
		ext = ".webp"
		if errors.Is(err, image.ErrFormat) {
			element.Error = apiException.FileNotImageError.Error()
			return element
		}
		if err != nil {
			element.Error = apiException.ServerError.Error()
			return element
		}
	}

	// 上传文件
	objectKey := objectService.GenerateObjectKey(data.Location, name, ext)
	err = bucket.SaveObject(reader, objectKey)
	if errors.Is(err, oss.ErrFileAlreadyExists) {
		element.Error = apiException.FileAlreadyExists.Error()
		return element
	}
	if err != nil {
		element.Error = apiException.ServerError.Error()
		return element
	}

	element.ObjectKey = objectKey
	zap.L().Info("上传文件成功", zap.String("bucket", data.Bucket), zap.String("objectKey", objectKey), zap.String("ip", ip))
	return element
}
