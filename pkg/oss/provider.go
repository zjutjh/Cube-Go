package oss

import (
	"errors"
	"io"
)

// StorageProvider 定义存储服务接口
type StorageProvider interface {
	SaveObject(reader io.ReadSeeker, objectKey string) error
	DeleteObject(objectKey string) error
	GetObject(objectKey string) (io.ReadCloser, *GetObjectInfo, error)
	GetFileList(prefix string) ([]FileListElement, error)
}

// FileListElement 文件列表元素
type FileListElement struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	Type         string `json:"type"`
	LastModified string `json:"last_modified"`
	ObjectKey    string `json:"object_key"`
}

// GetObjectInfo 获取对象内容
type GetObjectInfo struct {
	ContentType   string
	ContentLength int64
}

var (
	// ErrResourceNotExists 资源不存在
	ErrResourceNotExists = errors.New("resource not exists")
	// ErrPathIsNotDir 路径不是目录
	ErrPathIsNotDir = errors.New("path is not dir")
)
