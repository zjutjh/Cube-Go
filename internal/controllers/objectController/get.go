package objectController

import (
	"errors"
	"os"

	"github.com/gin-gonic/gin"
	"jh-oss/internal/apiException"
	"jh-oss/internal/services/objectService"
	"jh-oss/pkg/oss"
	"jh-oss/pkg/response"
)

type getFileListData struct {
	Bucket   string `form:"bucket" binding:"required"`
	Location string `form:"location"`
}

// GetFileList 获取文件列表
func GetFileList(c *gin.Context) {
	var data getFileListData
	if err := c.ShouldBindQuery(&data); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	bucket, err := oss.Buckets.GetBucket(data.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}

	loc := objectService.CleanLocation(data.Location)
	fileList, err := bucket.GetFileList(loc)
	if errors.Is(err, os.ErrNotExist) {
		apiException.AbortWithException(c, apiException.ResourceNotFound, err)
		return
	}
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	response.JsonSuccessResp(c, gin.H{
		"file_list": fileList,
	})
}
