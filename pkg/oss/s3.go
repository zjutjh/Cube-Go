package oss

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
func (p *S3StorageProvider) SaveObject(reader io.Reader, objectKey string) error {
	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return err
	}

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(p.bucketName),
		Key:    aws.String(objectKey),
		Body:   reader,
	})

	return err
}

// DeleteObject 删除对象
func (p *S3StorageProvider) DeleteObject(objectKey string) error {
	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return err
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
func (p *S3StorageProvider) GetFileList(prefix string) ([]FileListElement, error) {
	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return nil, err
	}

	result, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(p.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	fileList := make([]FileListElement, 0)
	for _, file := range result.Contents {
		fileList = append(fileList, FileListElement{
			LastModified: aws.ToTime(file.LastModified).Format(time.RFC3339),
			Name:         strings.TrimPrefix(aws.ToString(file.Key), prefix),
			ObjectKey:    aws.ToString(file.Key),
			Size:         aws.ToInt64(file.Size),
			Type:         p.getS3FileType(aws.ToString(file.Key)),
		})
	}

	return fileList, nil
}

func (p *S3StorageProvider) getS3FileType(objectKey string) string {
	client, err := s3Manager.GetConnection(p.target)
	if err != nil {
		return "binary"
	}

	headOutput, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(p.bucketName),
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
