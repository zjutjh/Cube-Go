package oss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/pkg/xattr"
	"go.uber.org/zap"
)

// LocalStorageProvider 本地存储提供者
type LocalStorageProvider struct {
	root *os.Root
}

// NewLocalStorageProvider 创建一个本地存储提供者
func NewLocalStorageProvider(p string) (*LocalStorageProvider, error) {
	folder := filepath.Join(".", p)
	if err := os.MkdirAll(folder, 0755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return nil, err
	}
	return &LocalStorageProvider{root: root}, nil
}

func (p *LocalStorageProvider) Close() error {
	return p.root.Close()
}

// SaveObject 保存对象到本地存储
func (p *LocalStorageProvider) SaveObject(ctx context.Context, reader io.ReadSeeker, objectKey string) error {
	key, isDir, err := NormalizeObjectKey(objectKey, false)
	if err != nil || isDir {
		return ErrInvalidObjectKey
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if dir := path.Dir(key); dir != "." {
		if err := p.root.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	outFile, err := p.root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, fs.ErrExist) {
		return ErrFileAlreadyExists
	}
	if err != nil {
		return err
	}
	removePartial := func() {
		_ = outFile.Close()
		_ = p.root.Remove(key)
	}
	if _, err = io.Copy(outFile, reader); err != nil {
		removePartial()
		return err
	}
	if err = ctx.Err(); err != nil {
		removePartial()
		return err
	}

	if xattr.XATTR_SUPPORTED {
		if _, err := reader.Seek(0, io.SeekStart); err == nil {
			if mime, err := mimetype.DetectReader(reader); err == nil {
				_ = xattr.FSet(outFile, "user.mimetype", []byte(mime.String()))
			}
		}
	}
	if err = outFile.Close(); err != nil {
		_ = p.root.Remove(key)
		return err
	}
	return nil
}

// DeleteObject 删除对象，目标不存在时仍视为成功。
func (p *LocalStorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	key, _, err := NormalizeObjectKey(objectKey, false)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.root.RemoveAll(key); err != nil {
		return err
	}

	for dir := path.Dir(key); dir != "."; dir = path.Dir(dir) {
		if err := p.root.Remove(dir); err != nil {
			break
		}
	}
	return nil
}

// GetObject 获取对象
func (p *LocalStorageProvider) GetObject(ctx context.Context, objectKey string, _ GetObjectOptions) (io.ReadCloser, *GetObjectInfo, error) {
	key, isDir, err := NormalizeObjectKey(objectKey, false)
	if err != nil || isDir {
		return nil, nil, ErrInvalidObjectKey
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	file, err := p.root.Open(key)
	if os.IsNotExist(err) {
		return nil, nil, ErrResourceNotExists
	}
	if err != nil {
		return nil, nil, err
	}
	info, err := objectInfoFromFile(file)
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if info == nil {
		_ = file.Close()
		return nil, nil, ErrResourceNotExists
	}
	return file, info, nil
}

func (p *LocalStorageProvider) StatObject(ctx context.Context, objectKey string, _ GetObjectOptions) (*GetObjectInfo, error) {
	reader, info, err := p.GetObject(ctx, objectKey, GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	_ = reader.Close()
	return info, nil
}

// GetFileList 获取文件列表
func (p *LocalStorageProvider) GetFileList(ctx context.Context, prefix string) ([]FileListElement, error) {
	key, _, err := NormalizeObjectKey(prefix, true)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target := key
	if target == "" {
		target = "."
	}
	stat, err := p.root.Stat(target)
	if os.IsNotExist(err) {
		return []FileListElement{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, ErrPathIsNotDir
	}

	entries, err := fs.ReadDir(p.root.FS(), target)
	if err != nil {
		return nil, err
	}
	list := make([]FileListElement, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fileInfo, err := entry.Info()
		if err != nil {
			zap.L().Error("获取文件信息错误", zap.Error(err))
			continue
		}
		objectKey := path.Join(key, fileInfo.Name())
		if entry.IsDir() {
			objectKey += "/"
		}
		list = append(list, FileListElement{
			Name:         fileInfo.Name(),
			Size:         fileInfo.Size(),
			Type:         p.getLocalFileType(objectKey, entry.IsDir()),
			LastModified: fileInfo.ModTime().Format(time.RFC3339),
			ObjectKey:    objectKey,
		})
	}
	return list, nil
}

func objectInfoFromFile(file *os.File) (*GetObjectInfo, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, nil
	}
	return &GetObjectInfo{
		ContentType:   getMimeType(file),
		ContentLength: stat.Size(),
		AcceptRanges:  "bytes",
		ETag:          fmt.Sprintf(`W/"%x-%x"`, stat.ModTime().UnixNano(), stat.Size()),
		LastModified:  stat.ModTime(),
	}, nil
}

func getMimeType(file *os.File) string {
	if xattr.XATTR_SUPPORTED {
		if value, err := xattr.FGet(file, "user.mimetype"); err == nil && len(value) > 0 {
			return string(value)
		}
	}
	_, _ = file.Seek(0, io.SeekStart)
	mime, err := mimetype.DetectReader(file)
	_, _ = file.Seek(0, io.SeekStart)
	if err != nil {
		return "application/octet-stream"
	}
	return mime.String()
}

func (p *LocalStorageProvider) getLocalFileType(objectKey string, isDir bool) string {
	if isDir {
		return "dir"
	}
	key, _, err := NormalizeObjectKey(objectKey, false)
	if err != nil {
		return "binary"
	}
	file, err := p.root.Open(key)
	if err != nil {
		return "binary"
	}
	defer func() { _ = file.Close() }()

	mimeType := getMimeType(file)
	switch {
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	case mimeType == "application/json":
		return "json"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	default:
		return "binary"
	}
}
