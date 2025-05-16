package oss

import (
	"context"
	"errors"
	"net/http"
	"sync"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3ConnectionManager S3连接管理器
type S3ConnectionManager struct {
	connections map[string]*s3.Client
	sync.RWMutex
}

// 定义连接相关错误
var (
	ErrConnectionAlreadyExists = errors.New("connection already exists")
	ErrConnectionNotFound      = errors.New("connection not found")
)

// AddConnection 添加连接
func (m *S3ConnectionManager) AddConnection(c *s3ConfigElement) error {
	if m.connections[c.Name] != nil {
		return ErrConnectionAlreadyExists
	}

	customTransport := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
		tr.TLSClientConfig.InsecureSkipVerify = !c.UseSSL
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithHTTPClient(customTransport),
		config.WithBaseEndpoint(c.Endpoint),
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKeyId, c.SecretAccessKey, "")),
	)
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = c.UsePathStyle })
	m.RWMutex.Lock()
	m.connections[c.Name] = client
	m.RWMutex.Unlock()
	return nil
}

// GetConnection 获取连接
func (m *S3ConnectionManager) GetConnection(name string) (*s3.Client, error) {
	m.RWMutex.RLock()
	defer m.RWMutex.RUnlock()

	client, ok := m.connections[name]
	if !ok {
		return nil, ErrConnectionNotFound
	}
	return client, nil
}
