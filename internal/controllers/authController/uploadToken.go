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

func UploadToken(c *gin.Context) {
	var form issueForm
	err := c.ShouldBindJSON(&form)
	if err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	_, err = oss.Buckets.GetBucket(form.Bucket)
	if err != nil {
		apiException.AbortWithException(c, apiException.BucketNotFound, err)
		return
	}
	loc, _, err := oss.NormalizeObjectKey(form.Location, true)
	if err != nil {
		apiException.AbortWithException(c, apiException.InvalidLocation, err)
		return
	}
	name := filepath.Base(form.Filename)
	if name == "" || name == "." || name == "/" {
		name = "file"
	}
	now := time.Now()
	claims := models.UploadClaims{
		Bucket:      form.Bucket,
		Location:    loc,
		Filename:    name,
		UseUUID:     form.UseUUID,
		ConvertWebP: form.ConvertWebP,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cube",
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
