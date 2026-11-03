package objectController

import (
	"cube-go/internal/apiException"
	"cube-go/internal/services/objectService"
	"cube-go/pkg/oss"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

// resolveName 解析实际写入的文件名（不含扩展名）与扩展名。
// want 为上传凭证指定的文件名，为空时沿用 multipart 里的文件名；
// useUUID 打开时忽略两者直接用 uuid；多文件上传时 want 追加 -序号（从 1 开始）。
func resolveName(fh *multipart.FileHeader, want string, useUUID bool, index, total int) (string, string) {
	ext := filepath.Ext(fh.Filename)
	name := strings.TrimSuffix(filepath.Base(fh.Filename), ext)
	if useUUID {
		return uuid.NewV1().String(), ext
	}
	if want != "" {
		name = strings.TrimSuffix(want, filepath.Ext(want))
		if total > 1 {
			name = fmt.Sprintf("%s-%d", name, index+1)
		}
	}
	if name == "" || name == "." || name == ".." {
		name = "file"
	}
	return name, ext
}

func saveUploaded(c *gin.Context, bucket oss.StorageProvider, fh *multipart.FileHeader,
	location, name, ext string, convertWebP bool) (string, error) {

	file, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	var reader io.ReadSeeker = file
	if convertWebP {
		reader, err = objectService.ConvertToWebP(file)
		ext = ".webp"
		if err != nil {
			return "", err
		}
	}

	objectKey, err := objectService.GenerateObjectKey(location, name, ext)
	if err != nil {
		return "", err
	}
	if err := bucket.SaveObject(c.Request.Context(), reader, objectKey); err != nil {
		return "", err
	}

	return objectKey, nil
}

func abortUploadErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, image.ErrFormat):
		apiException.AbortWithException(c, apiException.FileNotImageError, err)
	case errors.Is(err, oss.ErrInvalidObjectKey):
		apiException.AbortWithException(c, apiException.ParamError, err)
	case errors.Is(err, oss.ErrFileAlreadyExists):
		apiException.AbortWithException(c, apiException.FileAlreadyExists, err)
	default:
		apiException.AbortWithException(c, apiException.ServerError, err)
	}
}
