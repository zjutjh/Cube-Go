package objectController

import (
	"cube-go/internal/apiException"
	"cube-go/internal/services/objectService"
	"cube-go/pkg/oss"
	"errors"
	"image"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

func saveUploaded(c *gin.Context, bucket oss.StorageProvider, fh *multipart.FileHeader,
	location string, useUUID, convertWebP bool) (string, int64, error) {

	filename := fh.Filename
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	if useUUID {
		name = uuid.NewV1().String()
	}

	file, err := fh.Open()
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = file.Close() }()

	var reader io.ReadSeeker = file
	if convertWebP {
		reader, err = objectService.ConvertToWebP(file)
		ext = ".webp"
		if err != nil {
			return "", 0, err
		}
	}

	objectKey := objectService.GenerateObjectKey(location, name, ext)
	if err := bucket.SaveObject(c.Request.Context(), reader, objectKey); err != nil {
		return "", 0, err
	}

	return objectKey, fh.Size, nil
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
