package token

import (
	"cube-go/pkg/config"
)

// Key 返回 JWT 签名密钥：优先 oss.tokenKey，未配置则回落到 adminKey
func Key() []byte {
	if k := config.Config.GetString("oss.tokenKey"); k != "" {
		return []byte(k)
	}
	return []byte(config.Config.GetString("oss.adminKey"))
}
