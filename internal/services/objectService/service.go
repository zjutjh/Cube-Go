package objectService

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"cube-go/pkg/config"
	"cube-go/pkg/oss"
	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/dustin/go-humanize"
	_ "golang.org/x/image/webp" // 注册解码器
)

// SizeLimit 上传大小限制
var SizeLimit = humanize.MiByte * config.Config.GetInt64("oss.limit")

var maxLongEdge = config.Config.GetInt("oss.thumbnailLongEdge")

var invalidCharRegex = regexp.MustCompile(`[:*?"<>|]`)

var genLocks sync.Map

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// GenerateObjectKey 通过路径和文件名生成 ObjectKey
func GenerateObjectKey(location string, filename string, fileExt string) string {
	return path.Join(CleanLocation(location), filename+fileExt)
}

// CleanLocation 清理以避免非法路径
func CleanLocation(location string) string {
	isDir := strings.HasSuffix(location, "/")
	loc := invalidCharRegex.ReplaceAllString(location, "")
	result := strings.TrimLeft(path.Clean(loc), "./\\")
	if isDir {
		result += "/"
	}
	return result
}

// ConvertToWebP 将图片转换为 WebP 格式
func ConvertToWebP(reader io.Reader) (*bytes.Reader, error) {
	img, err := imaging.Decode(reader, imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}

	buf, _ := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// 编码为 WebP
	err = webp.Encode(buf, img, &webp.Options{
		Quality: float32(config.Config.GetInt("oss.quality")),
	})
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

// GetThumbnail 获取缩略图
func GetThumbnail(bucket string, objectKey string) (io.ReadCloser, *oss.GetObjectInfo, error) {
	filename := bucket + "-" + objectKey
	cachePath := filepath.Join(config.Config.GetString("oss.thumbnailDir"), filename+".jpg")

	// 尝试从缓存中读取
	if reader, info, err := readFromCache(cachePath); err == nil {
		return reader, info, nil
	}

	// 并发生成锁
	first, done := waitForPath(cachePath)
	if first {
		// 第一个进来的负责生成缩略图
		if err := generateThumbnail(bucket, objectKey, cachePath); err != nil {
			return nil, nil, err
		}
	}
	defer done()

	return readFromCache(cachePath)
}

// generateThumbnail 生成缩略图并写入缓存
func generateThumbnail(bucket, objectKey, cachePath string) error {
	// 从 OSS 获取源文件
	provider, err := oss.Buckets.GetBucket(bucket)
	if err != nil {
		return err
	}
	object, _, err := provider.GetObject(objectKey)
	if err != nil {
		return err
	}
	defer func() {
		_ = object.Close()
	}()

	// 解码图片
	img, err := imaging.Decode(object, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	img = removeAlpha(img)
	finalImg := imaging.Fit(img, maxLongEdge, maxLongEdge, imaging.CatmullRom)

	buf, _ := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	err = jpeg.Encode(buf, finalImg, &jpeg.Options{
		Quality: config.Config.GetInt("oss.thumbnailQuality"),
	})
	if err != nil {
		return err
	}

	// 写入缓存文件
	err = os.MkdirAll(filepath.Dir(cachePath), os.ModePerm)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, buf.Bytes(), 0644)
}

// readFromCache 从缓存文件读取
func readFromCache(cachePath string) (io.ReadCloser, *oss.GetObjectInfo, error) {
	stat, err := os.Stat(cachePath)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(cachePath)
	if err != nil {
		return nil, nil, err
	}
	info := &oss.GetObjectInfo{
		ContentType:   "image/jpeg",
		ContentLength: stat.Size(),
		LastModified:  stat.ModTime(),
	}
	return file, info, nil
}

// waitForPath 路径并发锁
func waitForPath(p string) (first bool, done func()) {
	ch := make(chan struct{})
	actual, loaded := genLocks.LoadOrStore(p, ch)
	if !loaded {
		// 第一个进来的
		return true, func() {
			close(ch)
			genLocks.Delete(p)
		}
	}
	// 不是第一个，就等待第一个完成
	if newCh, ok := actual.(chan struct{}); ok {
		<-newCh
	}
	return false, func() {}
}

// removeAlpha 去除 Alpha 通道，使用白底填充
func removeAlpha(img image.Image) image.Image {
	b := img.Bounds()
	bg := imaging.New(b.Dx(), b.Dy(), color.White)
	return imaging.Overlay(bg, img, image.Pt(0, 0), 1.0)
}
