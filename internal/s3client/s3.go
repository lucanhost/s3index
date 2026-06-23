package s3client

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/lucanhost/s3index/internal/config"
)

type Client struct {
	minioClient *minio.Client
	bucket      string
}

func NewClient(cfg *config.Config) (*Client, error) {
	host, secure := parseEndpoint(cfg.S3Endpoint)

	minioClient, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		minioClient: minioClient,
		bucket:      cfg.S3Bucket,
	}, nil
}

func parseEndpoint(endpoint string) (string, bool) {
	if endpoint == "" {
		return "s3.amazonaws.com", true
	}
	secure := true
	host := endpoint
	if strings.HasPrefix(endpoint, "https://") {
		host = strings.TrimPrefix(endpoint, "https://")
		secure = true
	} else if strings.HasPrefix(endpoint, "http://") {
		host = strings.TrimPrefix(endpoint, "http://")
		secure = false
	}
	return host, secure
}

func (c *Client) GetPresignedUrl(ctx context.Context, key string, expires time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := c.minioClient.PresignedGetObject(ctx, c.bucket, key, expires, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *Client) ListObjects(ctx context.Context, recursive bool) <-chan minio.ObjectInfo {
	return c.minioClient.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Recursive: recursive,
	})
}
