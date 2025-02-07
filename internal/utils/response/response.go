package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
