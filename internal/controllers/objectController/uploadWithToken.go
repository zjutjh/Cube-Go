package objectController

import (
	"cube-go/internal/apiException"
	"cube-go/internal/models"
	"cube-go/internal/services/objectService"
	"cube-go/internal/token"
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

const receiptTTL = 24 * time.Hour

// UploadWithToken 用上传凭证直传，桶/目录/文件名等参数以 token 为准
func UploadWithToken(c *gin.Context) {
	v, ok := c.Get("claims")
	if !ok {
		apiException.AbortWithException(c, apiException.TokenInvalid, nil)
		return
	}
	claims, ok := v.(*models.UploadClaims)
	if !ok || claims == nil {
		apiException.AbortWithException(c, apiException.TokenInvalid, nil)
		return
	}

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
		receipt, err := signReceipt(claims.Bucket, objectKey)
		if err != nil {
			apiException.AbortWithException(c, apiException.ServerError, err)
			return
		}
		files = append(files, gin.H{
			"object_key": objectKey,
			"url":        objectURL(claims.Bucket, objectKey),
			"receipt":    receipt,
		})
	}

	response.JsonSuccessResp(c, gin.H{"files": files})
}

// signReceipt 签发上传回执，供业务后端验真
func signReceipt(bucket, objectKey string) (string, error) {
	now := time.Now()
	claims := models.ReceiptClaims{
		Bucket:    bucket,
		ObjectKey: objectKey,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    token.Issuer,
			Audience:  jwt.ClaimStrings{token.AudienceReceipt},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(receiptTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(token.Key())
}

// objectURL 拼下载地址，沿用旧 query 入口
func objectURL(bucket, objectKey string) string {
	q := url.Values{}
	q.Set("bucket", bucket)
	q.Set("object_key", objectKey)
	return "/api/file?" + q.Encode()
}
