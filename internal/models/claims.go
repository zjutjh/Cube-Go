package models

import (
	"github.com/golang-jwt/jwt/v5"
)

type UploadClaims struct {
	Bucket      string `json:"bkt"`
	Location    string `json:"loc"`
	Filename    string `json:"fnm"`
	UseUUID     bool   `json:"uuid"`
	ConvertWebP bool   `json:"webp"`
	jwt.RegisteredClaims
}

type ReceiptClaims struct {
	Bucket    string `json:"bkt"`
	ObjectKey string `json:"key"`
	Size      int64  `json:"size"`
	ETag      string `json:"etag,omitempty"`
	jwt.RegisteredClaims
}
