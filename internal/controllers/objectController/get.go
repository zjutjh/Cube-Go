package objectController

import (
	"errors"
	"net/http"

	"cube-go/internal/apiException"
	"cube-go/internal/services/objectService"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"
	"github.com/gin-gonic/gin"
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
	if errors.Is(err, oss.ErrPathIsNotDir) {
		apiException.AbortWithException(c, apiException.ParamError, err)
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

// GetBucketList 获取存储桶列表
func GetBucketList(c *gin.Context) {
	response.JsonSuccessResp(c, gin.H{
		"bucket_list": oss.Buckets.GetBucketList(),
	})
}
