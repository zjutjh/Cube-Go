package oss

import (
	"sync"

	"cube-go/pkg/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var s3Manager = S3ConnectionManager{
	connections: make(map[string]*s3.Client),
	RWMutex:     sync.RWMutex{},
}

type s3ConfigElement struct {
	Name            string `mapstructure:"name"`
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyId     string `mapstructure:"accessKeyId"`
	SecretAccessKey string `mapstructure:"secretAccessKey"`
	Region          string `mapstructure:"region"`
	UseSSL          bool   `mapstructure:"useSSL"`
	UsePathStyle    bool   `mapstructure:"usePathStyle"`
}

// InitS3Connections 初始化S3连接
func initS3Connections() error {
	var cfgList []s3ConfigElement
	err := config.Config.UnmarshalKey("s3", &cfgList)
	if err != nil {
		return err
	}

	for _, c := range cfgList {
		err := s3Manager.AddConnection(&c)
		if err != nil {
			return err
		}
	}
	return nil
}
