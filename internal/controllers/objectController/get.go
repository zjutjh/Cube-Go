package objectController

import (
	"errors"
	"net/http"

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

type getFileData struct {
	Bucket    string `form:"bucket" binding:"required"`
	ObjectKey string `form:"object_key" binding:"required"`
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
	if errors.Is(err, oss.ErrResourceNotExists) {
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

// GetFile 下载文件
func GetFile(c *gin.Context) {
	var data getFileData
	if err := c.ShouldBindQuery(&data); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	bucket, err := oss.Buckets.GetBucket(data.Bucket)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	obj, content, err := bucket.GetObject(data.ObjectKey)
	if errors.Is(err, oss.ErrResourceNotExists) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = obj.Close()
	}()

	c.DataFromReader(http.StatusOK, content.ContentLength, content.ContentType, obj, nil)
}
