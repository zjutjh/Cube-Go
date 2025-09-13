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
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/draw"
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
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	img, _ = fixOrientation(img, bytes.NewReader(data))

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

// fixOrientation 修复图片的旋转
func fixOrientation(img image.Image, exifData io.Reader) (image.Image, error) {
	x, err := exif.Decode(exifData)
	if err != nil {
		return nil, err
	}

	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return nil, err
	}

	orientation, err := tag.Int(0)
	if err != nil {
		return nil, err
	}

	return applyOrientation(img, orientation), nil
}

// applyOrientation 根据 EXIF Orientation 调整图像方向
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 1: // 正常
		return img
	case 2: // 水平翻转
		return imaging.FlipH(img)
	case 3: // 旋转 180°
		return imaging.Rotate180(img)
	case 4: // 垂直翻转
		return imaging.FlipV(img)
	case 5: // 顺时针 90° + 水平翻转
		return imaging.FlipH(imaging.Rotate270(img))
	case 6: // 顺时针 90°
		return imaging.Rotate270(img)
	case 7: // 顺时针 90° + 垂直翻转
		return imaging.FlipV(imaging.Rotate270(img))
	case 8: // 逆时针 90°
		return imaging.Rotate90(img)
	default:
		return img
	}
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
	img, err := decodeImg(object)
	if err != nil {
		return nil, 0, err
	}
	img = removeAlpha(img)
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

func removeAlpha(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(b)

	draw.Draw(out, b, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(out, b, img, b.Min, draw.Over)
	return out
}
