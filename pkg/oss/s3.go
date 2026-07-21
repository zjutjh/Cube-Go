package oss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gabriel-vasile/mimetype"
)

// S3StorageProvider S3存储提供者
type S3StorageProvider struct {
	client     *s3.Client
	bucketName string
}

// NewS3StorageProvider 创建S3存储提供者
func NewS3StorageProvider(client *s3.Client, bucketName string) StorageProvider {
	return &S3StorageProvider{client: client, bucketName: bucketName}
}

// SaveObject 存储对象
func (p *S3StorageProvider) SaveObject(ctx context.Context, reader io.ReadSeeker, objectKey string) error {
	key, isDir, err := NormalizeObjectKey(objectKey, false)
	if err != nil || isDir {
		return ErrInvalidObjectKey
	}
	mime, err := mimetype.DetectReader(reader)
	if err != nil {
		return err
	}
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		return err
	}

	_, err = p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.bucketName),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(mime.String()),
		IfNoneMatch: aws.String("*"),
	})
	if errors.Is(mapS3Error(err), ErrPreconditionFailed) {
		return ErrFileAlreadyExists
	}
	return mapS3Error(err)
}

// DeleteObject 删除对象，目标不存在时仍视为成功。
func (p *S3StorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	key, isDir, err := NormalizeObjectKey(objectKey, false)
	if err != nil {
		return err
	}
	if isDir {
		err = deleteFolderContents(ctx, p.client, p.bucketName, key+"/")
	} else {
		_, err = p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(p.bucketName),
			Key:    aws.String(key),
		})
	}
	mappedErr := mapS3Error(err)
	if errors.Is(mappedErr, ErrResourceNotExists) {
		return nil
	}
	return mappedErr
}

// GetObject 获取对象
func (p *S3StorageProvider) GetObject(ctx context.Context, objectKey string, options GetObjectOptions) (io.ReadCloser, *GetObjectInfo, error) {
	key, isDir, err := NormalizeObjectKey(objectKey, false)
	if err != nil || isDir {
		return nil, nil, ErrInvalidObjectKey
	}
	input := &s3.GetObjectInput{
		Bucket: aws.String(p.bucketName),
		Key:    aws.String(key),
		Range:  optionalString(options.Range),
	}
	applyGetConditions(input, options.Conditions)
	result, err := p.client.GetObject(ctx, input)
	if err != nil {
		return nil, nil, mapS3Error(err)
	}
	return result.Body, &GetObjectInfo{
		ContentLength: aws.ToInt64(result.ContentLength),
		ContentRange:  aws.ToString(result.ContentRange),
		ContentType:   aws.ToString(result.ContentType),
		AcceptRanges:  "bytes",
		ETag:          aws.ToString(result.ETag),
		LastModified:  aws.ToTime(result.LastModified),
	}, nil
}

func (p *S3StorageProvider) StatObject(ctx context.Context, objectKey string, options GetObjectOptions) (*GetObjectInfo, error) {
	key, isDir, err := NormalizeObjectKey(objectKey, false)
	if err != nil || isDir {
		return nil, ErrInvalidObjectKey
	}
	input := &s3.HeadObjectInput{
		Bucket: aws.String(p.bucketName),
		Key:    aws.String(key),
		Range:  optionalString(options.Range),
	}
	applyHeadConditions(input, options.Conditions)
	result, err := p.client.HeadObject(ctx, input)
	if err != nil {
		return nil, mapS3Error(err)
	}
	return &GetObjectInfo{
		ContentLength: aws.ToInt64(result.ContentLength),
		ContentRange:  aws.ToString(result.ContentRange),
		ContentType:   aws.ToString(result.ContentType),
		AcceptRanges:  "bytes",
		ETag:          aws.ToString(result.ETag),
		LastModified:  aws.ToTime(result.LastModified),
	}, nil
}

// GetFileList 获取文件列表
func (p *S3StorageProvider) GetFileList(ctx context.Context, requestedPrefix string) ([]FileListElement, error) {
	key, _, err := NormalizeObjectKey(requestedPrefix, true)
	if err != nil {
		return nil, err
	}
	prefix := key
	if prefix != "" {
		prefix += "/"
	}
	paginator := s3.NewListObjectsV2Paginator(p.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(p.bucketName),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	fileList := make([]FileListElement, 0)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, mapS3Error(err)
		}
		for _, common := range page.CommonPrefixes {
			commonPrefix := aws.ToString(common.Prefix)
			fileList = append(fileList, FileListElement{
				Name:      strings.TrimSuffix(strings.TrimPrefix(commonPrefix, prefix), "/"),
				ObjectKey: commonPrefix,
				Type:      "dir",
			})
		}
		for _, file := range page.Contents {
			objectKey := aws.ToString(file.Key)
			name := strings.TrimPrefix(objectKey, prefix)
			if strings.Contains(name, "/") {
				continue
			}
			fileList = append(fileList, FileListElement{
				LastModified: aws.ToTime(file.LastModified).Local().Format(time.RFC3339),
				Name:         name,
				ObjectKey:    objectKey,
				Size:         aws.ToInt64(file.Size),
				Type:         getS3FileType(ctx, p.client, p.bucketName, objectKey),
			})
		}
	}
	return fileList, nil
}

func getS3FileType(ctx context.Context, client *s3.Client, bucketName, objectKey string) string {
	result, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return "binary"
	}
	return classifyMIME(aws.ToString(result.ContentType))
}

func classifyMIME(mimeType string) string {
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

func deleteFolderContents(ctx context.Context, client *s3.Client, bucketName, prefix string) error {
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return mapS3Error(err)
		}
		if len(page.Contents) == 0 {
			continue
		}
		objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, object := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{Key: object.Key})
		}
		result, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucketName),
			Delete: &types.Delete{Objects: objects},
		})
		if err != nil {
			return mapS3Error(err)
		}
		if len(result.Errors) > 0 {
			deleteErrs := make([]error, 0, len(result.Errors))
			for _, item := range result.Errors {
				deleteErrs = append(deleteErrs, fmt.Errorf("delete %q: %s: %s", aws.ToString(item.Key), aws.ToString(item.Code), aws.ToString(item.Message)))
			}
			return errors.Join(deleteErrs...)
		}
	}
	return nil
}

func applyGetConditions(input *s3.GetObjectInput, conditions ObjectConditions) {
	input.IfMatch = optionalString(conditions.IfMatch)
	input.IfNoneMatch = optionalString(conditions.IfNoneMatch)
	input.IfModifiedSince = conditions.IfModifiedSince
	input.IfUnmodifiedSince = conditions.IfUnmodifiedSince
}

func applyHeadConditions(input *s3.HeadObjectInput, conditions ObjectConditions) {
	input.IfMatch = optionalString(conditions.IfMatch)
	input.IfNoneMatch = optionalString(conditions.IfNoneMatch)
	input.IfModifiedSince = conditions.IfModifiedSince
	input.IfUnmodifiedSince = conditions.IfUnmodifiedSince
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return aws.String(value)
}

func mapS3Error(err error) error {
	if err == nil {
		return nil
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return ErrResourceNotExists
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchBucket":
			return err
		case "NoSuchKey", "NotFound":
			return ErrResourceNotExists
		case "NotModified":
			return wrapS3ResponseError(err, ErrNotModified)
		case "PreconditionFailed":
			return wrapS3ResponseError(err, ErrPreconditionFailed)
		case "InvalidRange", "RequestedRangeNotSatisfiable":
			return wrapS3ResponseError(err, ErrInvalidRange)
		}
	}
	switch s3StatusCode(err) {
	case 304:
		return wrapS3ResponseError(err, ErrNotModified)
	case 404:
		return ErrResourceNotExists
	case 412:
		return wrapS3ResponseError(err, ErrPreconditionFailed)
	case 416:
		return wrapS3ResponseError(err, ErrInvalidRange)
	default:
		return err
	}
}

func wrapS3ResponseError(source, mapped error) error {
	var responseErr *smithyhttp.ResponseError
	if !errors.As(source, &responseErr) || responseErr.HTTPResponse() == nil {
		return mapped
	}
	response := responseErr.HTTPResponse().Response
	info := &GetObjectInfo{
		ContentType:   response.Header.Get("Content-Type"),
		ContentLength: response.ContentLength,
		ContentRange:  response.Header.Get("Content-Range"),
		AcceptRanges:  response.Header.Get("Accept-Ranges"),
		ETag:          response.Header.Get("ETag"),
	}
	if info.ContentLength < 0 {
		if length, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64); err == nil {
			info.ContentLength = length
		}
	}
	if modified, err := http.ParseTime(response.Header.Get("Last-Modified")); err == nil {
		info.LastModified = modified
	}
	return &ObjectResponseError{Err: mapped, Info: info}
}

func s3StatusCode(err error) int {
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		return responseErr.HTTPStatusCode()
	}
	return 0
}
