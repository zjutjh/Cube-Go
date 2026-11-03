package token

import (
	"cube-go/pkg/config"
)

const (
	// Issuer 本服务签发的 JWT issuer
	Issuer = "cube"
	// AudienceUpload 直传凭证
	AudienceUpload = "upload"
	// AudienceReceipt 上传回执
	AudienceReceipt = "receipt"
)

// Key 返回 JWT 签名密钥：优先 oss.tokenKey，未配置则回落到 adminKey
func Key() []byte {
	if k := config.Config.GetString("oss.tokenKey"); k != "" {
		return []byte(k)
	}
	return []byte(config.Config.GetString("oss.adminKey"))
}
