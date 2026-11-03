package authController

import (
	"cube-go/internal/apiException"
	"cube-go/internal/models"
	"cube-go/internal/token"
	"cube-go/pkg/oss"
	"cube-go/pkg/response"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 300 * time.Second

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
	now := time.Now()
	claims := models.UploadClaims{
		Bucket:      form.Bucket,
		Location:    loc,
		Filename:    cleanFilename(form.Filename),
		UseUUID:     form.UseUUID,
		ConvertWebP: form.ConvertWebP,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    token.Issuer,
			Audience:  jwt.ClaimStrings{token.AudienceUpload},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(token.Key())
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	response.JsonSuccessResp(c, gin.H{
		"token":      signed,
		"expires_at": claims.ExpiresAt.Unix(),
	})
}

// cleanFilename 归一化文件名，空串表示上传时沿用 multipart 里的文件名
func cleanFilename(name string) string {
	if name == "" {
		return ""
	}
	base := filepath.Base(name)
	if base == "." || base == ".." || base == "/" {
		return ""
	}
	return base
}
