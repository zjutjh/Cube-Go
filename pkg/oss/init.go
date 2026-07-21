package oss

import (
	"context"
	"errors"

	"cube-go/pkg/config"
)

type bucketConfigElement struct {
	Name       string `mapstructure:"name"`
	Type       string `mapstructure:"type"`
	Target     string `mapstructure:"target"`
	BucketName string `mapstructure:"bucketName"`
	Path       string `mapstructure:"path"`
}

// Buckets 全局桶管理器
var Buckets = &BucketManager{
	buckets: make(map[string]StorageProvider),
}

var (
	// ErrUnknownBucketType 未知桶类型
	ErrUnknownBucketType = errors.New("unknown bucket type")
)

// Init 初始化OSS
func Init(ctx context.Context) error {
	connections, err := initS3Connections(ctx)
	if err != nil {
		return err
	}

	var cfgList []bucketConfigElement
	err = config.Config.UnmarshalKey("bucket", &cfgList)
	if err != nil {
		return err
	}

	buckets := make(map[string]StorageProvider, len(cfgList))
	manager := &BucketManager{buckets: buckets}
	for _, c := range cfgList {
		if _, exists := buckets[c.Name]; exists {
			_ = manager.Close()
			return ErrBucketAlreadyExists
		}

		var provider StorageProvider
		if c.Type == "s3" {
			client, exists := connections[c.Target]
			if !exists {
				_ = manager.Close()
				return ErrConnectionNotFound
			}
			provider = NewS3StorageProvider(client, c.BucketName)
		} else if c.Type == "local" {
			provider, err = NewLocalStorageProvider(c.Path)
			if err != nil {
				_ = manager.Close()
				return err
			}
		} else {
			_ = manager.Close()
			return ErrUnknownBucketType
		}
		buckets[c.Name] = provider
	}
	Buckets = manager
	return nil
}

func Close() error {
	return Buckets.Close()
}
