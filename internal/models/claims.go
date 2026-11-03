package models

import (
	"github.com/golang-jwt/jwt/v5"
)

// AudienceUpload / AudienceReceipt 区分凭证类型，避免回执被当作凭证复用
const (
	AudienceUpload  = "upload"
	AudienceReceipt = "receipt"
)

// UploadClaims 直传凭证，桶/目录/文件名等参数以 token 为准
type UploadClaims struct {
	Bucket      string `json:"bucket"`
	Location    string `json:"location"`
	Filename    string `json:"filename"` // 为空时沿用 multipart 里的文件名
	UseUUID     bool   `json:"use_uuid"`
	ConvertWebP bool   `json:"convert_webp"`
	jwt.RegisteredClaims
}

// ReceiptClaims 上传回执，供业务后端验真
type ReceiptClaims struct {
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
	jwt.RegisteredClaims
}
