package apiException

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/pkg/log"
)

// Error 自定义错误类型
type Error struct {
	Code  int
	Msg   string
	Level log.Level
}

// 自定义错误
var (
	ServerError         = NewError(200500, log.LevelError, "系统异常，请稍后重试")
	ParamError          = NewError(200501, log.LevelInfo, "参数错误")
	UploadFileError     = NewError(200502, log.LevelError, "上传文件失败")
	FileSizeExceedError = NewError(200503, log.LevelInfo, "文件大小超限")
	FileNotImageError   = NewError(200504, log.LevelInfo, "上传的文件不是图片")
	ResourceNotFound    = NewError(200505, log.LevelInfo, "资源不存在")
	NoPermission        = NewError(200506, log.LevelInfo, "权限不足")
	BucketNotFound      = NewError(200508, log.LevelInfo, "存储桶不存在")

	NotFound = NewError(200404, log.LevelWarn, http.StatusText(http.StatusNotFound))
)

// Error 实现 error 接口，返回错误的消息内容
func (e *Error) Error() string {
	return e.Msg
}

// NewError 创建并返回一个新的自定义错误实例
func NewError(code int, level log.Level, msg string) *Error {
	return &Error{
		Code:  code,
		Msg:   msg,
		Level: level,
	}
}

// AbortWithException 用于返回自定义错误信息
func AbortWithException(c *gin.Context, apiError *Error, err error) {
	logError(c, apiError, err)
	_ = c.AbortWithError(200, apiError)
}

// logError 记录错误日志
func logError(c *gin.Context, apiErr *Error, err error) {
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
