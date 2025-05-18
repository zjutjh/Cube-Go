package oss

import (
	"io"
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
	path string
}

// NewLocalStorageProvider 创建一个本地存储提供者
func NewLocalStorageProvider(p string) StorageProvider {
	folder := filepath.Join("./", p)
	_ = os.MkdirAll(folder, os.ModePerm)

	return &LocalStorageProvider{
		path: folder,
	}
}

// SaveObject 保存对象到本地存储
func (p *LocalStorageProvider) SaveObject(reader io.Reader, objectKey string) error {
	// 根据 objectKey 解析出文件的路径
	relativePath := filepath.Join(p.path, objectKey)

	// 创建文件夹，如果文件夹不存在
	err := os.MkdirAll(filepath.Dir(relativePath), os.ModePerm)
	if err != nil {
		return err
	}

	// 创建文件
	outFile, err := os.Create(relativePath)
	if err != nil {
		return err
	}

	// 写入文件
	_, err = io.Copy(outFile, reader)
	if err != nil {
		return err
	}
	_ = outFile.Close()

	// 尝试保存 MIME 类型到 xattr
	if xattr.XATTR_SUPPORTED {
		mime, err := mimetype.DetectFile(relativePath)
		if err == nil {
			_ = xattr.Set(relativePath, "user.mimetype", []byte(mime.String()))
		}
	}
	return nil
}

// DeleteObject 删除对象
func (p *LocalStorageProvider) DeleteObject(objectKey string) error {
	// 根据 objectKey 解析出文件的路径
	relativePath := filepath.Join(p.path, objectKey)

	// 检查文件是否存在
	_, err := os.Stat(relativePath)
	if os.IsNotExist(err) {
		return ErrResourceNotExists
	}

	// 删除文件
	err = os.RemoveAll(relativePath)
	return err
}

// GetObject 获取对象
func (p *LocalStorageProvider) GetObject(objectKey string) (io.ReadCloser, *GetObjectInfo, error) {
	// 根据 objectKey 解析出文件路径
	relativePath := filepath.Join(p.path, objectKey)

	// 检查文件是否存在
	stat, err := os.Stat(relativePath)
	if os.IsNotExist(err) || stat.IsDir() {
		return nil, nil, ErrResourceNotExists
	}
	if err != nil {
		return nil, nil, err
	}

	// 读取文件
	file, err := os.Open(relativePath)
	if err != nil {
		return nil, nil, err
	}

	info := &GetObjectInfo{
		ContentLength: stat.Size(),
		ContentType:   getMimeType(relativePath),
	}
	return file, info, nil
}

// GetFileList 获取文件列表
func (p *LocalStorageProvider) GetFileList(prefix string) ([]FileListElement, error) {
	filePath := filepath.Join(p.path, prefix)
	stat, err := os.Stat(filePath)
	if os.IsNotExist(err) || !stat.IsDir() {
		return nil, ErrResourceNotExists
	}

	fileList, err := os.ReadDir(filePath)
	if err != nil {
		return nil, err
	}

	list := make([]FileListElement, 0)
	for _, file := range fileList {
		fileInfo, err := file.Info()
		if err != nil {
			zap.L().Error("获取文件信息错误", zap.Error(err))
			continue
		}

		key := path.Join(prefix, fileInfo.Name())
		if file.IsDir() {
			key += "/"
		}
		list = append(list, FileListElement{
			Name:         fileInfo.Name(),
			Size:         fileInfo.Size(),
			Type:         getLocalFileType(filepath.Join(filePath, fileInfo.Name()), file.IsDir()),
			LastModified: fileInfo.ModTime().Format(time.RFC3339),
			ObjectKey:    key,
		})
	}
	return list, nil
}

func getMimeType(filePath string) string {
	if xattr.XATTR_SUPPORTED {
		t, err := xattr.Get(filePath, "user.mimetype")
		if err == nil && len(t) > 0 {
			return string(t)
		}
	}
	mime, err := mimetype.DetectFile(filePath)
	if err != nil {
		return "application/octet-stream"
	}
	return mime.String()
}

func getLocalFileType(filePath string, isDir bool) string {
	if isDir {
		return "dir"
	}

	mimeType := getMimeType(filePath)
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
