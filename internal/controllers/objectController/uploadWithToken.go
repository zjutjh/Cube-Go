package objectController

import (
	"cube-go/internal/apiException"
	"cube-go/internal/models"
	"cube-go/internal/services/objectService"
	"cube-go/pkg/config"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// UploadWithToken 用直传凭证上传，桶/目录/文件名等参数以 token 为准
func UploadWithToken(c *gin.Context) {
	claims := c.MustGet("claims").(*models.UploadClaims)

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, objectService.SizeLimit)

	var form struct {
		Files []*multipart.FileHeader `form:"file" binding:"required"`
	}
	if err := c.ShouldBind(&form); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			apiException.AbortWithException(c, apiException.FileSizeExceedError, err)
			return
		}
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	bucket, err := oss.Buckets.GetBucket(claims.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}

	files := make([]gin.H, 0, len(form.Files))
	for i, fh := range form.Files {
		name, ext := resolveName(fh, claims.Filename, claims.UseUUID, i, len(form.Files))
		objectKey, err := saveUploaded(c, bucket, fh, claims.Location, name, ext, claims.ConvertWebP)
		if err != nil {
			// 与 /api/upload 一致：遇到第一个错误就中断，不回滚已写入的文件
			abortUploadErr(c, err)
			return
		}

		now := time.Now()
		receipt, err := jwt.NewWithClaims(jwt.SigningMethodHS256, models.ReceiptClaims{
			Bucket:    claims.Bucket,
			ObjectKey: objectKey,
			RegisteredClaims: jwt.RegisteredClaims{
				Audience:  jwt.ClaimStrings{models.AudienceReceipt},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			},
		}).SignedString([]byte(config.Config.GetString("oss.adminKey")))
		if err != nil {
			apiException.AbortWithException(c, apiException.ServerError, err)
			return
		}

		files = append(files, gin.H{
			"object_key": objectKey,
			"url":        "/api/file?" + url.Values{"bucket": {claims.Bucket}, "object_key": {objectKey}}.Encode(),
			"receipt":    receipt,
		})
	}

	response.JsonSuccessResp(c, gin.H{"files": files})
}
