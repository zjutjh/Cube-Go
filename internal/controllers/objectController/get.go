package objectController

import (
	"errors"
	"image"
	"net/http"
	"time"

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
	Thumbnail bool   `form:"thumbnail"`
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
	objectKey := objectService.CleanLocation(data.ObjectKey)

	if data.Thumbnail {
		thumbnail, info, err := objectService.GetThumbnail(data.Bucket, objectKey)
		if errors.Is(err, oss.ErrResourceNotExists) || errors.Is(err, image.ErrFormat) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = thumbnail.Close()
		}()

		if checkCacheHeaders(c, info.LastModified) {
			return
		}

		c.DataFromReader(http.StatusOK, info.ContentLength, info.ContentType, thumbnail, nil)
	} else {
		bucket, err := oss.Buckets.GetBucket(data.Bucket)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		obj, content, err := bucket.GetObject(objectKey)
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

		if checkCacheHeaders(c, content.LastModified) {
			return
		}

		c.DataFromReader(http.StatusOK, content.ContentLength, content.ContentType, obj, nil)
	}
}

// checkCacheHeaders 检查 If-Modified-Since 并设置缓存头部
func checkCacheHeaders(c *gin.Context, lastModified time.Time) bool {
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("Last-Modified", lastModified.Format(http.TimeFormat))

	ifModifiedSince := c.GetHeader("If-Modified-Since")
	if ifModifiedSince != "" {
		if t, err := time.Parse(http.TimeFormat, ifModifiedSince); err == nil {
			// 忽略毫秒，只精确到秒进行比较
			if !lastModified.Truncate(time.Second).After(t) {
				c.Status(http.StatusNotModified)
				return true
			}
		}
	}
	return false
}

// GetBucketList 获取存储桶列表
func GetBucketList(c *gin.Context) {
	response.JsonSuccessResp(c, gin.H{
		"bucket_list": oss.Buckets.GetBucketList(),
	})
}
