package objectService

import (
	"bytes"
	"image"
	_ "image/gif" // 注册解码器
	"image/jpeg"
	_ "image/png" // 注册解码器
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
	"github.com/dustin/go-humanize"
	_ "golang.org/x/image/bmp" // 注册解码器
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff" // 注册解码器
	_ "golang.org/x/image/webp"
)

// SizeLimit 上传大小限制
var SizeLimit = humanize.MiByte * config.Config.GetInt64("oss.limit")

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
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	buf, _ := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	err = webp.Encode(buf, img, &webp.Options{
		Quality: float32(config.Config.GetInt("oss.quality")),
	})
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

// GetThumbnail 获取缩略图
func GetThumbnail(bucket string, objectKey string) (io.ReadCloser, int64, error) {
	filename := bucket + "-" + objectKey
	cachePath := filepath.Join(config.Config.GetString("oss.thumbnailDir"), filename+".jpg")

	// 尝试从缓存中读取
	if stat, err := os.Stat(cachePath); err == nil {
		file, err := os.Open(cachePath)
		if err == nil {
			return file, stat.Size(), nil
		}
	}

	// 并发生成锁
	first, done := waitForPath(cachePath)
	if !first {
		// 别人已经在生成，直接等它生成完再读缓存
		if stat, err := os.Stat(cachePath); err == nil {
			file, err := os.Open(cachePath)
			if err == nil {
				return file, stat.Size(), nil
			}
		}
		return nil, 0, oss.ErrResourceNotExists
	}
	defer done()

	// 从 OSS 获取源文件
	provider, err := oss.Buckets.GetBucket(bucket)
	if err != nil {
		return nil, 0, err
	}
	object, _, err := provider.GetObject(objectKey)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = object.Close()
	}()

	// 解码图片
	img, _, err := image.Decode(object)
	if err != nil {
		return nil, 0, err
	}
	finalImg := resizeIfNeeded(img, config.Config.GetInt("oss.thumbnailLongEdge"))

	buf, _ := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	err = jpeg.Encode(buf, finalImg, &jpeg.Options{
		Quality: config.Config.GetInt("oss.thumbnailQuality"),
	})
	if err != nil {
		return nil, 0, err
	}

	// 写入缓存文件
	if err := os.MkdirAll(filepath.Dir(cachePath), os.ModePerm); err == nil {
		_ = os.WriteFile(cachePath, buf.Bytes(), 0644)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), int64(buf.Len()), nil
}

func resizeIfNeeded(img image.Image, targetLongSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	longSide, shortSide := max(w, h), min(w, h)

	if longSide <= targetLongSide {
		return img
	}
	scale := float64(targetLongSide) / float64(longSide)
	targetShort := int(float64(shortSide) * scale)

	var tw, th int
	if w > h {
		tw, th = targetLongSide, targetShort
	} else {
		tw, th = targetShort, targetLongSide
	}

	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.BiLinear.Scale(dst, dst.Rect, img, b, draw.Over, nil)
	return dst
}

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
