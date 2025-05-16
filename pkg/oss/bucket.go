package oss

import (
	"errors"
	"sync"
)

// BucketManager 存储桶管理器
type BucketManager struct {
	buckets map[string]StorageProvider
	sync.RWMutex
}

// 定义存储桶相关错误
var (
	ErrBucketAlreadyExists = errors.New("bucket already exists")
	ErrBucketNotFound      = errors.New("bucket not found")
)

// AddBucket 添加存储桶
func (m *BucketManager) AddBucket(name string, c StorageProvider) error {
	m.RWMutex.Lock()
	defer m.RWMutex.Unlock()
	if _, ok := m.buckets[name]; ok {
		return ErrBucketAlreadyExists
	}
	m.buckets[name] = c
	return nil
}

// GetBucket 获取存储桶
func (m *BucketManager) GetBucket(name string) (StorageProvider, error) {
	m.RWMutex.RLock()
	defer m.RWMutex.RUnlock()
	if c, ok := m.buckets[name]; ok {
		return c, nil
	}
	return nil, ErrBucketNotFound
}

// GetBucketList 获取存储桶列表
func (m *BucketManager) GetBucketList() []string {
	m.RWMutex.RLock()
	defer m.RWMutex.RUnlock()
	list := make([]string, 0, len(m.buckets))
	for k := range m.buckets {
		list = append(list, k)
	}
	return list
}
