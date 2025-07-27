package midwares

import (
	"errors"
	"net/http"

	"cube-go/internal/apiException"
	"cube-go/pkg/config"
	"cube-go/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrHandler 中间件用于处理请求错误
// 如果存在错误，将返回相应的 JSON 响应
func ErrHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 向下执行请求
		c.Next()

		// 如果存在错误，则处理错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if err != nil {
				var apiErr *apiException.Error

				if errors.As(err, &apiErr) {
					response.JsonErrorResp(c, apiErr.Code, apiErr.Msg)
				} else {
					zap.L().Error("Error occurred", zap.Error(err))
				}
				return
			}
		}
	}
}

// HandleNotFound 处理 404 错误。
func HandleNotFound(c *gin.Context) {
	err := apiException.NotFound
	// 记录 404 错误日志
	zap.L().Warn("404 Not Found",
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
	)
	response.JsonResp(c, http.StatusNotFound, err.Code, err.Msg, nil)
}

// Auth 验证权限
func Auth(c *gin.Context) {
	key := c.GetHeader("Key")
	if key != config.Config.GetString("oss.adminKey") { // 验证权限
		apiException.AbortWithException(c, apiException.NoPermission, nil)
		return
	}
	c.Next()
}
