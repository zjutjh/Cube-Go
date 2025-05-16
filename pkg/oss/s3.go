package oss

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gabriel-vasile/mimetype"
)

// S3StorageProvider S3存储提供者
type S3StorageProvider struct {
	target     string
	bucketName string
}

// NewS3StorageProvider 创建S3存储提供者
func NewS3StorageProvider(target string, bucketName string) StorageProvider {
	return &S3StorageProvider{
		target:     target,
		bucketName: bucketName,
	}
}

// SaveObject 存储对象
func (p *S3StorageProvider) SaveObject(r io.Reader, objectKey string) error {
	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return err
	}

	// 缓存数据以获取文件类型
	data, _ := io.ReadAll(r)
	buf := bytes.NewBuffer(data)
	mime, err := mimetype.DetectReader(buf)
	if err != nil {
		return err
	}

	// 重置指针到开头供后续使用
	reader := bytes.NewReader(data)

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(p.bucketName),
		Key:         aws.String(objectKey),
		Body:        reader,
		ContentType: aws.String(mime.String()),
	})

	return err
}

// DeleteObject 删除对象
func (p *S3StorageProvider) DeleteObject(objectKey string) error {
	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return err
	}

	// 如果为文件夹
	if strings.HasSuffix(objectKey, "/") {
		return deleteFolderContents(client, p.bucketName, objectKey)
	}

	_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucketName),
		Key:    aws.String(objectKey),
	})
	return err
}

// GetObject 获取对象
func (p *S3StorageProvider) GetObject(objectKey string) (io.ReadCloser, error) {
	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return nil, err
	}

	result, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(p.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, err
	}

	return result.Body, nil
}

// GetFileList 获取文件列表
func (p *S3StorageProvider) GetFileList(pf string) ([]FileListElement, error) {
	prefix := pf
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return nil, err
	}

	result, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket:    aws.String(p.bucketName),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // 限制当前层级
	})
	if err != nil {
		return nil, err
	}

	fileList := make([]FileListElement, 0)

	// 处理子文件夹
	for _, cp := range result.CommonPrefixes {
		commonPrefix := aws.ToString(cp.Prefix)
		folderName := strings.TrimSuffix(strings.TrimPrefix(commonPrefix, prefix), "/")
		fileList = append(fileList, FileListElement{
			LastModified: "",
			Name:         folderName,
			ObjectKey:    commonPrefix,
			Size:         0,
			Type:         "dir",
		})
	}

	// 处理文件
	for _, file := range result.Contents {
		key := aws.ToString(file.Key)
		name := strings.TrimPrefix(key, prefix)

		// 检查是否属于当前层级（去掉 prefix 后不包含 '/'）
		if strings.Contains(name, "/") {
			continue // 跳过子文件夹中的文件
		}

		fileList = append(fileList, FileListElement{
			LastModified: aws.ToTime(file.LastModified).Format(time.RFC3339),
			Name:         name,
			ObjectKey:    key,
			Size:         aws.ToInt64(file.Size),
			Type:         getS3FileType(client, p.bucketName, key),
		})
	}

	return fileList, nil
}

func getS3FileType(client *s3.Client, bucketName string, objectKey string) string {
	headOutput, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return "binary"
	}

	mimeType := aws.ToString(headOutput.ContentType)
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

func deleteFolderContents(client *s3.Client, bucketName string, prefix string) error {
	// 列出文件夹下所有对象
	listResult, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return err
	}

	if len(listResult.Contents) == 0 {
		return nil // 文件夹为空
	}

	// 构建删除对象列表
	objects := make([]types.ObjectIdentifier, 0)
	for _, obj := range listResult.Contents {
		objects = append(objects, types.ObjectIdentifier{Key: obj.Key})
	}

	// 批量删除
	_, err = client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{Objects: objects},
	})
	return err
}
