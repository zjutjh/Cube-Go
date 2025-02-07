package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/apiException"
	"jh-oss/pkg/log"
)

// JsonResp 返回json格式数据
func JsonResp(c *gin.Context, httpStatusCode int, code int, msg string, data any) {
	c.JSON(httpStatusCode, gin.H{
		"code": code,
		"msg":  msg,
		"data": data,
	})
}

// JsonSuccessResp 返回成功json格式数据
func JsonSuccessResp(c *gin.Context, data any) {
	JsonResp(c, http.StatusOK, 200, "OK", data)
}

// JsonErrorResp 返回错误json格式数据
func JsonErrorResp(c *gin.Context, code int, msg string) {
	JsonResp(c, http.StatusOK, code, msg, nil)
}

// AbortWithException 用于返回自定义错误信息
func AbortWithException(c *gin.Context, apiError *apiException.Error, err error) {
	logError(c, apiError, err)
	_ = c.AbortWithError(200, apiError)
}

// logError 记录错误日志
func logError(c *gin.Context, apiErr *apiException.Error, err error) {
	// 构建日志字段
	logFields := []zap.Field{
		zap.Int("error_code", apiErr.Code),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("ip", c.ClientIP()),
		zap.Error(err), // 记录原始错误信息
	}
	log.GetLogFunc(apiErr.Level)(apiErr.Msg, logFields...)
}
