package objectController

import (
	"cube-go/internal/apiException"
	"cube-go/internal/models"
	"cube-go/internal/services/objectService"
	"cube-go/internal/token"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

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

	// 只绑 file，绝不绑 bucket/location
	var form struct {
		File *multipart.FileHeader `form:"file" binding:"required"`
	}
	if err := c.ShouldBind(&form); err != nil {
		// MaxBytesError → FileSizeExceedError，其余 → ParamError
		return
	}

	bucket, err := oss.Buckets.GetBucket(claims.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
	}

	objectKey, size, err := saveUploaded(c, bucket, form.File, claims.Location, claims.UseUUID, claims.ConvertWebP)
	if err != nil {
		abortUploadErr(c, err)
		return
	}
	// 签回执
	now := time.Now()
	rc := models.ReceiptClaims{
		Bucket:    claims.Bucket,
		ObjectKey: objectKey,
		Size:      size,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cube",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}
	receipt, err := jwt.NewWithClaims(jwt.SigningMethodHS256, rc).SignedString(token.Key())
	url := "/api/file?bucket=" + url.QueryEscape(claims.Bucket) + "&object_key=" + url.QueryEscape(objectKey)
	// 返回 object_key / url / size / receipt

	response.JsonSuccessResp(c, gin.H{
		"object_key": objectKey,
		"url":        url,
		"size":       size,
		"receipt":    receipt,
	})
}
