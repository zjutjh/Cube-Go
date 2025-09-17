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
	"time"

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
	if stat, err := os.Stat(cachePath); err == nil {
		file, err := os.Open(cachePath)
		if err == nil {
			info := oss.GetObjectInfo{
				ContentType:   "image/jpeg",
				ContentLength: stat.Size(),
				LastModified:  stat.ModTime(),
			}
			return file, &info, nil
		}
	}

	// 并发生成锁
	first, done := waitForPath(cachePath)
	if !first {
		// 别人已经在生成，直接等它生成完再读缓存
		if stat, err := os.Stat(cachePath); err == nil {
			file, err := os.Open(cachePath)
			if err == nil {
				info := oss.GetObjectInfo{
					ContentType:   "image/jpeg",
					ContentLength: stat.Size(),
					LastModified:  stat.ModTime(),
				}
				return file, &info, nil
			}
		}
		return nil, nil, oss.ErrResourceNotExists
	}
	defer done()

	// 从 OSS 获取源文件
	provider, err := oss.Buckets.GetBucket(bucket)
	if err != nil {
		return nil, nil, err
	}
	object, _, err := provider.GetObject(objectKey)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = object.Close()
	}()

	// 解码图片
	img, err := imaging.Decode(object, imaging.AutoOrientation(true))
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	// 写入缓存文件
	if err := os.MkdirAll(filepath.Dir(cachePath), os.ModePerm); err == nil {
		_ = os.WriteFile(cachePath, buf.Bytes(), 0644)
	}

	info := oss.GetObjectInfo{
		ContentType:   "image/jpeg",
		ContentLength: int64(buf.Len()),
		LastModified:  time.Now(),
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), &info, nil
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

func removeAlpha(img image.Image) image.Image {
	b := img.Bounds()
	bg := imaging.New(b.Dx(), b.Dy(), color.White)
	return imaging.Overlay(bg, img, image.Pt(0, 0), 1.0)
}
