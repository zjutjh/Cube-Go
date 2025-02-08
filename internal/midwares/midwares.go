package midwares

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/apiException"
	"jh-oss/pkg/response"
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

				// 尝试将错误转换为 Exception
				ok := errors.As(err, &apiErr)

				// 如果转换失败，则使用 ServerError
				if !ok {
					apiErr = apiException.ServerError
					zap.L().Error("Unknown error occurred", zap.Error(err))
				}

				response.JsonErrorResp(c, apiErr.Code, apiErr.Msg)
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
