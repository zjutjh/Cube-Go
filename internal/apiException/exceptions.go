package apiException

import (
	"net/http"

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
	ServerError = NewError(200500, log.LevelError, "系统异常，请稍后重试")
	ParamsError = NewError(200501, log.LevelInfo, "参数错误")
	NotFound    = NewError(200404, log.LevelWarn, http.StatusText(http.StatusNotFound))
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
