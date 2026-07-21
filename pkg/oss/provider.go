package oss

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"time"
)

// StorageProvider 定义存储服务接口
type StorageProvider interface {
	SaveObject(ctx context.Context, reader io.ReadSeeker, objectKey string) error
	DeleteObject(ctx context.Context, objectKey string) error
	GetObject(ctx context.Context, objectKey string, options GetObjectOptions) (io.ReadCloser, *GetObjectInfo, error)
	StatObject(ctx context.Context, objectKey string, options GetObjectOptions) (*GetObjectInfo, error)
	GetFileList(ctx context.Context, prefix string) ([]FileListElement, error)
}

type ObjectConditions struct {
	IfMatch           string
	IfNoneMatch       string
	IfModifiedSince   *time.Time
	IfUnmodifiedSince *time.Time
}

type GetObjectOptions struct {
	Conditions ObjectConditions
	Range      string
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
	ContentRange  string
	AcceptRanges  string
	ETag          string
	LastModified  time.Time
}

type ObjectResponseError struct {
	Err  error
	Info *GetObjectInfo
}

func (e *ObjectResponseError) Error() string {
	return e.Err.Error()
}

func (e *ObjectResponseError) Unwrap() error {
	return e.Err
}

var (
	// ErrResourceNotExists 资源不存在
	ErrResourceNotExists = errors.New("resource not exists")
	// ErrPathIsNotDir 路径不是目录
	ErrPathIsNotDir = errors.New("path is not dir")
	// ErrFileAlreadyExists 文件已存在
	ErrFileAlreadyExists = errors.New("file already exists")
	// ErrInvalidObjectKey 对象键不合法
	ErrInvalidObjectKey = errors.New("invalid object key")
	// ErrNotModified 对象未修改
	ErrNotModified = errors.New("not modified")
	// ErrPreconditionFailed 请求前置条件不满足
	ErrPreconditionFailed = errors.New("precondition failed")
	// ErrInvalidRange 请求范围不合法
	ErrInvalidRange = errors.New("invalid range")
)

// NormalizeObjectKey 将对象键规范为不带尾斜杠的相对路径。
func NormalizeObjectKey(objectKey string, allowEmpty bool) (string, bool, error) {
	isDir := strings.HasSuffix(objectKey, "/")
	key := strings.TrimSuffix(objectKey, "/")
	if key == "" && allowEmpty {
		return "", isDir, nil
	}
	if key == "." || strings.Contains(key, `\`) || !fs.ValidPath(key) {
		return "", false, ErrInvalidObjectKey
	}
	return key, isDir, nil
}
