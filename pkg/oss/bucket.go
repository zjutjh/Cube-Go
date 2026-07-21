package oss

import (
	"errors"
	"io"
	"sort"
)

// BucketManager 存储桶管理器
type BucketManager struct {
	buckets map[string]StorageProvider
}

// 定义存储桶相关错误
var (
	ErrBucketAlreadyExists = errors.New("bucket already exists")
	ErrBucketNotFound      = errors.New("bucket not found")
)

// GetBucket 获取存储桶
func (m *BucketManager) GetBucket(name string) (StorageProvider, error) {
	if c, ok := m.buckets[name]; ok {
		return c, nil
	}
	return nil, ErrBucketNotFound
}

// GetBucketList 获取存储桶列表
func (m *BucketManager) GetBucketList() []string {
	list := make([]string, 0, len(m.buckets))
	for k := range m.buckets {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

func (m *BucketManager) Close() error {
	var errs []error
	for _, bucket := range m.buckets {
		if closer, ok := bucket.(io.Closer); ok {
			errs = append(errs, closer.Close())
		}
	}
	return errors.Join(errs...)
}
