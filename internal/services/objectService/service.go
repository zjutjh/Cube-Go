package objectService

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

	"github.com/disintegration/imaging"
	"github.com/dustin/go-humanize"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
)

// SizeLimit 上传大小限制
var SizeLimit = humanize.MiByte * config.Config.GetInt64("oss.limit")

var maxLongEdge = config.Config.GetInt("oss.thumbnailLongEdge")

var invalidCharRegex = regexp.MustCompile(`[:*?"<>|]`)

var genLocks sync.Map

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

	var buf bytes.Buffer

	// 编码为 WebP
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, float32(config.Config.GetInt("oss.quality")))
	if err != nil {
		return nil, err
	}
	err = webp.Encode(&buf, img, options)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

// GetThumbnail 获取缩略图
func GetThumbnail(ctx context.Context, bucket string, objectKey string) (io.ReadCloser, *oss.GetObjectInfo, error) {
	provider, err := oss.Buckets.GetBucket(bucket)
	if err != nil {
		return nil, nil, err
	}
	sourceInfo, err := provider.StatObject(ctx, objectKey, oss.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}
	cacheKey := thumbnailCacheKey(bucket, objectKey, sourceInfo)
	cachePath := filepath.Join(config.Config.GetString("oss.thumbnailDir"), cacheKey+".jpg")

	// 尝试从缓存中读取
	if reader, info, err := readFromCache(cachePath, sourceInfo, cacheKey); err == nil {
		return reader, info, nil
	}

	// 并发生成锁
	first, done := waitForPath(cachePath)
	defer done()
	if reader, info, err := readFromCache(cachePath, sourceInfo, cacheKey); err == nil {
		return reader, info, nil
	}
	if first {
		if err := generateThumbnail(ctx, provider, objectKey, cachePath, sourceInfo); err != nil {
			return nil, nil, err
		}
	}

	return readFromCache(cachePath, sourceInfo, cacheKey)
}

// generateThumbnail 生成缩略图并写入缓存
func generateThumbnail(ctx context.Context, provider oss.StorageProvider, objectKey, cachePath string, sourceInfo *oss.GetObjectInfo) error {
	object, actualInfo, err := provider.GetObject(ctx, objectKey, oss.GetObjectOptions{
		Conditions: oss.ObjectConditions{IfMatch: sourceInfo.ETag},
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = object.Close()
	}()
	if sourceInfo.ETag != "" {
		if actualInfo.ETag != sourceInfo.ETag {
			return oss.ErrPreconditionFailed
		}
	} else if actualInfo.ContentLength != sourceInfo.ContentLength || !actualInfo.LastModified.Equal(sourceInfo.LastModified) {
		return oss.ErrPreconditionFailed
	}

	// 解码图片
	img, err := imaging.Decode(object, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	img = removeAlpha(img)
	finalImg := imaging.Fit(img, maxLongEdge, maxLongEdge, imaging.CatmullRom)

	var buf bytes.Buffer

	err = jpeg.Encode(&buf, finalImg, &jpeg.Options{
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
	temp, err := os.CreateTemp(filepath.Dir(cachePath), ".thumbnail-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	if err := temp.Chmod(0644); err != nil {
		return err
	}
	if _, err := temp.Write(buf.Bytes()); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), cachePath)
}

// readFromCache 从缓存文件读取
func readFromCache(cachePath string, sourceInfo *oss.GetObjectInfo, cacheKey string) (io.ReadCloser, *oss.GetObjectInfo, error) {
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
		AcceptRanges:  "bytes",
		ETag:          `"` + cacheKey + `"`,
		LastModified:  sourceInfo.LastModified,
	}
	return file, info, nil
}

func thumbnailCacheKey(bucket, objectKey string, info *oss.GetObjectInfo) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "jpeg-v1\x00%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00%d", bucket, objectKey, info.ETag,
		info.LastModified.UTC().UnixNano(), info.ContentLength, maxLongEdge, config.Config.GetInt("oss.thumbnailQuality"))
	return hex.EncodeToString(hash.Sum(nil))
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
