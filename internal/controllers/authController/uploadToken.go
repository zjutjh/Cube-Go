package authController

import (
	"cube-go/internal/apiException"
	"cube-go/internal/models"
	"cube-go/pkg/config"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type issueForm struct {
	Bucket      string `json:"bucket" binding:"required"`
	Location    string `json:"location"`
	Filename    string `json:"filename"`
	ConvertWebP bool   `json:"convert_webp"`
	UseUUID     bool   `json:"use_uuid"`
}

// UploadToken 签发直传凭证
func UploadToken(c *gin.Context) {
	var form issueForm
	if err := c.ShouldBindJSON(&form); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	if _, err := oss.Buckets.GetBucket(form.Bucket); err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}
	loc, _, err := oss.NormalizeObjectKey(form.Location, true)
	if err != nil {
		apiException.AbortWithException(c, apiException.InvalidLocation, err)
		return
	}
	// 空串表示上传时沿用 multipart 里的文件名
	filename := ""
	if base := filepath.Base(form.Filename); base != "." && base != ".." {
		filename = base
	}
	now := time.Now()
	claims := models.UploadClaims{
		Bucket:      form.Bucket,
		Location:    loc,
		Filename:    filename,
		UseUUID:     form.UseUUID,
		ConvertWebP: form.ConvertWebP,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{models.AudienceUpload},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(300 * time.Second)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.Config.GetString("oss.adminKey")))
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	response.JsonSuccessResp(c, gin.H{
		"token":      signed,
		"expires_at": claims.ExpiresAt.Unix(),
	})
}
