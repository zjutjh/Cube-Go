package oss

import (
	"context"
	"errors"

	"cube-go/pkg/config"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3ConfigElement struct {
	Name            string `mapstructure:"name"`
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyId     string `mapstructure:"accessKeyId"`
	SecretAccessKey string `mapstructure:"secretAccessKey"`
	Region          string `mapstructure:"region"`
	UsePathStyle    bool   `mapstructure:"usePathStyle"`
}

// InitS3Connections 初始化S3连接
func initS3Connections(ctx context.Context) (map[string]*s3.Client, error) {
	var cfgList []s3ConfigElement
	err := config.Config.UnmarshalKey("s3", &cfgList)
	if err != nil {
		return nil, err
	}

	connections := make(map[string]*s3.Client, len(cfgList))
	for _, c := range cfgList {
		if _, exists := connections[c.Name]; exists {
			return nil, ErrConnectionAlreadyExists
		}
		client, err := newS3Connection(ctx, &c)
		if err != nil {
			return nil, err
		}
		connections[c.Name] = client
	}
	return connections, nil
}

var (
	ErrConnectionAlreadyExists = errors.New("connection already exists")
	ErrConnectionNotFound      = errors.New("connection not found")
)
