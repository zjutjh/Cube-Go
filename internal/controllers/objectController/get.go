package objectController

import (
	"errors"
	"image"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
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
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")

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
	fileList, err := bucket.GetFileList(c.Request.Context(), loc)
	if errors.Is(err, oss.ErrPathIsNotDir) || errors.Is(err, oss.ErrInvalidObjectKey) {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	response.JsonSuccessResp(c, gin.H{"file_list": fileList})
}

// GetFile 将旧 query 下载接口重定向到唯一的资源路径。
func GetFile(c *gin.Context) {
	var data getFileData
	if err := c.ShouldBindQuery(&data); err != nil || strings.ContainsAny(data.Bucket, `/\`) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	objectKey, isDir, err := oss.NormalizeObjectKey(data.ObjectKey, false)
	if err != nil || isDir {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	prefix := "/files/"
	if data.Thumbnail {
		prefix = "/thumbnails/"
	}
	target := (&url.URL{Path: prefix + data.Bucket + "/" + objectKey}).String()
	c.Redirect(http.StatusPermanentRedirect, target)
}

func ServeFile(c *gin.Context) {
	serveObject(c, false)
}

func ServeThumbnail(c *gin.Context) {
	serveObject(c, true)
}

func serveObject(c *gin.Context, thumbnail bool) {
	bucketName := c.Param("bucket")
	objectKey, isDir, err := oss.NormalizeObjectKey(strings.TrimPrefix(c.Param("object_key"), "/"), false)
	if err != nil || isDir {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	bucket, err := oss.Buckets.GetBucket(bucketName)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, no-cache")

	if thumbnail {
		reader, info, err := objectService.GetThumbnail(c.Request.Context(), bucketName, objectKey)
		if err != nil {
			handleObjectError(c, err)
			return
		}
		serveSeekable(c, objectKey+".jpg", reader, info)
		return
	}
	if _, local := bucket.(*oss.LocalStorageProvider); local {
		reader, info, err := bucket.GetObject(c.Request.Context(), objectKey, oss.GetObjectOptions{})
		if err != nil {
			handleObjectError(c, err)
			return
		}
		serveSeekable(c, objectKey, reader, info)
		return
	}
	serveRemoteObject(c, bucket, objectKey)
}

func serveSeekable(c *gin.Context, name string, reader io.ReadCloser, info *oss.GetObjectInfo) {
	defer func() { _ = reader.Close() }()
	seeker, ok := reader.(io.ReadSeeker)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	setObjectHeaders(c, info, false)
	http.ServeContent(c.Writer, c.Request, path.Base(name), info.LastModified.UTC(), seeker)
}

func serveRemoteObject(c *gin.Context, bucket oss.StorageProvider, objectKey string) {
	options := oss.GetObjectOptions{Conditions: requestConditions(c.Request)}
	options.Range = c.GetHeader("Range")
	if strings.Contains(options.Range, ",") {
		handleRemoteObjectError(c, bucket, objectKey, oss.ErrInvalidRange)
		return
	}
	if options.Range != "" && c.GetHeader("If-Range") != "" {
		info, err := bucket.StatObject(c.Request.Context(), objectKey, oss.GetObjectOptions{Conditions: options.Conditions})
		if err != nil {
			handleRemoteObjectError(c, bucket, objectKey, err)
			return
		}
		if !ifRangeMatches(c.GetHeader("If-Range"), info) {
			options.Range = ""
		} else {
			options.Conditions = oss.ObjectConditions{IfMatch: info.ETag}
		}
	}

	if c.Request.Method == http.MethodHead {
		var info *oss.GetObjectInfo
		var err error
		if options.Range == "" {
			info, err = bucket.StatObject(c.Request.Context(), objectKey, options)
		} else {
			var reader io.ReadCloser
			reader, info, err = bucket.GetObject(c.Request.Context(), objectKey, options)
			if reader != nil {
				_ = reader.Close()
			}
		}
		if err != nil {
			handleRemoteObjectError(c, bucket, objectKey, err)
			return
		}
		partial := info.ContentRange != ""
		setObjectHeaders(c, info, true)
		if partial {
			c.Status(http.StatusPartialContent)
		} else {
			c.Status(http.StatusOK)
		}
		return
	}

	reader, info, err := bucket.GetObject(c.Request.Context(), objectKey, options)
	if err != nil {
		handleRemoteObjectError(c, bucket, objectKey, err)
		return
	}
	defer func() { _ = reader.Close() }()
	partial := info.ContentRange != ""
	setObjectHeaders(c, info, true)
	if partial {
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}
	_, _ = io.Copy(c.Writer, reader)
}

func handleRemoteObjectError(c *gin.Context, bucket oss.StorageProvider, objectKey string, err error) {
	if !errors.Is(err, oss.ErrNotModified) && !errors.Is(err, oss.ErrInvalidRange) {
		handleObjectError(c, err)
		return
	}

	var responseErr *oss.ObjectResponseError
	var info *oss.GetObjectInfo
	if errors.As(err, &responseErr) {
		info = responseErr.Info
	}
	needStat := info == nil || info.ETag == "" || info.LastModified.IsZero()
	if errors.Is(err, oss.ErrInvalidRange) && (info == nil || info.ContentRange == "") {
		needStat = true
	}
	if needStat {
		statInfo, statErr := bucket.StatObject(c.Request.Context(), objectKey, oss.GetObjectOptions{})
		if statErr == nil {
			if info == nil {
				info = statInfo
			} else {
				if info.ETag == "" {
					info.ETag = statInfo.ETag
				}
				if info.LastModified.IsZero() {
					info.LastModified = statInfo.LastModified
				}
				if info.ContentLength < 0 {
					info.ContentLength = statInfo.ContentLength
				}
				if errors.Is(err, oss.ErrInvalidRange) && info.ContentRange == "" {
					info.ContentLength = statInfo.ContentLength
				}
				if info.AcceptRanges == "" {
					info.AcceptRanges = statInfo.AcceptRanges
				}
			}
		}
	}
	if info != nil {
		if errors.Is(err, oss.ErrInvalidRange) && info.ContentRange == "" && info.ContentLength >= 0 {
			info.ContentRange = "bytes */" + strconv.FormatInt(info.ContentLength, 10)
		}
		setObjectHeaders(c, info, false)
	}
	if errors.Is(err, oss.ErrNotModified) {
		c.Status(http.StatusNotModified)
	} else {
		c.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
	}
}

func requestConditions(request *http.Request) oss.ObjectConditions {
	conditions := oss.ObjectConditions{
		IfMatch:     request.Header.Get("If-Match"),
		IfNoneMatch: request.Header.Get("If-None-Match"),
	}
	if value, err := http.ParseTime(request.Header.Get("If-Modified-Since")); err == nil {
		conditions.IfModifiedSince = &value
	}
	if value, err := http.ParseTime(request.Header.Get("If-Unmodified-Since")); err == nil {
		conditions.IfUnmodifiedSince = &value
	}
	return conditions
}

func ifRangeMatches(value string, info *oss.GetObjectInfo) bool {
	if strings.HasPrefix(value, `"`) {
		return !strings.HasPrefix(info.ETag, "W/") && value == info.ETag
	}
	t, err := http.ParseTime(value)
	return err == nil && !info.LastModified.UTC().Truncate(time.Second).After(t)
}

func setObjectHeaders(c *gin.Context, info *oss.GetObjectInfo, includeLength bool) {
	if info.ContentType != "" {
		c.Header("Content-Type", info.ContentType)
	}
	if info.AcceptRanges != "" {
		c.Header("Accept-Ranges", info.AcceptRanges)
	}
	if info.ContentRange != "" {
		c.Header("Content-Range", info.ContentRange)
	}
	if info.ETag != "" {
		c.Header("ETag", info.ETag)
	}
	if !info.LastModified.IsZero() {
		c.Header("Last-Modified", info.LastModified.UTC().Truncate(time.Second).Format(http.TimeFormat))
	}
	if includeLength && info.ContentLength >= 0 {
		c.Header("Content-Length", strconv.FormatInt(info.ContentLength, 10))
	}
}

func handleObjectError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, oss.ErrResourceNotExists), errors.Is(err, image.ErrFormat):
		c.AbortWithStatus(http.StatusNotFound)
	case errors.Is(err, oss.ErrInvalidObjectKey):
		c.AbortWithStatus(http.StatusBadRequest)
	case errors.Is(err, oss.ErrNotModified):
		c.Status(http.StatusNotModified)
	case errors.Is(err, oss.ErrPreconditionFailed):
		c.AbortWithStatus(http.StatusPreconditionFailed)
	case errors.Is(err, oss.ErrInvalidRange):
		c.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
	default:
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

// GetBucketList 获取存储桶列表
func GetBucketList(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	response.JsonSuccessResp(c, gin.H{"bucket_list": oss.Buckets.GetBucketList()})
}
