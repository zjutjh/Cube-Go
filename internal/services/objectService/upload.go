package objectService

import (
	"bytes"
	"image"
	_ "image/gif" // 注册解码器
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	_ "golang.org/x/image/bmp" // 注册解码器
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"jh-oss/pkg/config"
)

// SizeLimit 上传大小限制
var SizeLimit = humanize.MByte * config.Config.GetInt64("oss.limit")

// GenerateObjectKey 通过路径和文件名生成 ObjectKey
func GenerateObjectKey(location string, filename string, fileExt string) string {
	return path.Join(CleanLocation(location), filename+fileExt)
}

// CleanLocation 清理以避免非法路径
func CleanLocation(location string) string {
	return strings.TrimLeft(path.Clean(location), "./\\")
}

// SaveObject 根据 ObjectKey 保存文件
func SaveObject(reader io.Reader, objectKey string) error {
	// 根据 objectKey 解析出文件的路径
	relativePath := filepath.Join("static", objectKey)

	// 提取目录部分
	dir := filepath.Dir(relativePath)

	// 创建文件夹，如果文件夹不存在
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return err
	}

	// 创建文件
	outFile, err := os.Create(relativePath)
	if err != nil {
		return err
	}
	defer func(outFile *os.File) {
		err := outFile.Close()
		if err != nil {
			zap.L().Warn("文件关闭错误", zap.Error(err))
		}
	}(outFile)

	// 写入文件
	_, err = io.Copy(outFile, reader)
	return err
}

// ConvertToWebP 将图片转换为 WebP 格式
func ConvertToWebP(reader io.Reader) (io.Reader, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = webp.Encode(&buf, img, &webp.Options{Quality: 100})
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}
