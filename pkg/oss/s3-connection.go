package oss

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func newS3Connection(ctx context.Context, c *s3ConfigElement) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithBaseEndpoint(c.Endpoint),
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKeyId, c.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = c.UsePathStyle })
	return client, nil
}
