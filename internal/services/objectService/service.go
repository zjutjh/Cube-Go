package objectService

import (
	"bytes"
	"image"
	_ "image/gif" // 注册解码器
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path"
	"strings"

	"github.com/chai2010/webp"
	"github.com/dustin/go-humanize"
	_ "golang.org/x/image/bmp" // 注册解码器
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"jh-oss/pkg/config"
)

// SizeLimit 上传大小限制
var SizeLimit = humanize.MiByte * config.Config.GetInt64("oss.limit")

// GenerateObjectKey 通过路径和文件名生成 ObjectKey
func GenerateObjectKey(location string, filename string, fileExt string) string {
	return path.Join(CleanLocation(location), filename+fileExt)
}

// CleanLocation 清理以避免非法路径
func CleanLocation(location string) string {
	isDir := strings.HasSuffix(location, "/")
	loc := location
	invalidChars := []string{":", "*", "?", "<", ">", "|", "\""}
	for _, char := range invalidChars {
		loc = strings.ReplaceAll(loc, char, "")
	}

	result := strings.TrimLeft(path.Clean(loc), "./\\")
	if isDir {
		result += "/"
	}
	return result
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
