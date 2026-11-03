package midwares

import (
	"cube-go/internal/apiException"
	"cube-go/internal/models"
	"cube-go/pkg/config"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// TokenAuth 校验直传凭证
func TokenAuth(c *gin.Context) {
	raw, ok := strings.CutPrefix(strings.TrimSpace(c.GetHeader("Authorization")), "Bearer ")
	if !ok || raw == "" {
		apiException.AbortWithException(c, apiException.TokenInvalid, nil)
		return
	}

	claims := &models.UploadClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return []byte(config.Config.GetString("oss.adminKey")), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithAudience(models.AudienceUpload), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			apiException.AbortWithException(c, apiException.TokenExpired, err)
			return
		}
		apiException.AbortWithException(c, apiException.TokenInvalid, err)
		return
	}
	c.Set("claims", claims)
	c.Next()
}
