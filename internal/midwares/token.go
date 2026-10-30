package midwares

import (
	"cube-go/internal/apiException"
	"cube-go/internal/models"
	"cube-go/internal/token"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const bearerPrefix = "Bearer "

func TokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, bearerPrefix) {
			apiException.AbortWithException(c, apiException.TokenInvalid, nil)
			return
		}
		raw := strings.TrimSpace(strings.TrimPrefix(auth, bearerPrefix))
		if raw == "" {
			apiException.AbortWithException(c, apiException.TokenInvalid, nil)
			return
		}

		claims := &models.UploadClaims{}
		_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			return token.Key(), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
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
}
