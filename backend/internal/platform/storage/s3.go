package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store stores objects in any S3-compatible service (AWS S3, Cloudflare R2, MinIO).
type S3Store struct {
	client *s3.Client
	bucket string
}

// S3Config configures NewS3. Endpoint is empty for AWS S3; set it (and PathStyle) for MinIO/R2.
// Static credentials are for local MinIO only; production uses the default AWS credential chain.
type S3Config struct {
	Endpoint, Bucket, Region string
	PathStyle                bool
	AccessKey, SecretKey     string
}

func NewS3(ctx context.Context, c S3Config) (*S3Store, error) {
	opts := []func(*config.LoadOptions) error{config.WithRegion(c.Region)}
	if c.AccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("storage: aws config: %w", err)
	}
	cl := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
		o.UsePathStyle = c.PathStyle
	})
	return &S3Store{client: cl, bucket: c.Bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, r io.Reader) error {
	// Buffer: PutObject needs a seekable body for payload signing; reports and raw batches are small.
	b, err := io.ReadAll(io.LimitReader(r, 256<<20))
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: bytesReader(b)})
	return err
}

func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}
