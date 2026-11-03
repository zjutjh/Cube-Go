package models

import (
	"github.com/golang-jwt/jwt/v5"
)

// UploadClaims 直传凭证，桶/目录/文件名等参数以 token 为准
type UploadClaims struct {
	Bucket      string `json:"bkt"`
	Location    string `json:"loc"`
	Filename    string `json:"fnm"` // 为空时沿用 multipart 里的文件名
	UseUUID     bool   `json:"uuid"`
	ConvertWebP bool   `json:"webp"`
	jwt.RegisteredClaims
}

// ReceiptClaims 上传回执，供业务后端验真
type ReceiptClaims struct {
	Bucket    string `json:"bkt"`
	ObjectKey string `json:"key"`
	jwt.RegisteredClaims
}
