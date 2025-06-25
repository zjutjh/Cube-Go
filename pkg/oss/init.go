package oss

import (
	"errors"
	"sync"

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
var Buckets = BucketManager{
	buckets: make(map[string]StorageProvider),
	RWMutex: sync.RWMutex{},
}

var (
	// ErrUnknownBucketType 未知桶类型
	ErrUnknownBucketType = errors.New("unknown bucket type")
)

// Init 初始化OSS
func Init() error {
	err := initS3Connections()
	if err != nil {
		return err
	}

	var cfgList []bucketConfigElement
	err = config.Config.UnmarshalKey("bucket", &cfgList)
	if err != nil {
		return err
	}

	for _, c := range cfgList {
		if c.Type == "s3" {
			err := Buckets.AddBucket(c.Name, NewS3StorageProvider(c.Target, c.BucketName))
			if err != nil {
				return err
			}
		} else if c.Type == "local" {
			err := Buckets.AddBucket(c.Name, NewLocalStorageProvider(c.Path))
			if err != nil {
				return err
			}
		} else {
			return ErrUnknownBucketType
		}
	}
	return nil
}
