package objectstorage

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
)

type s3Service struct {
	client *s3.Client
	logger *zap.Logger
	scope  tally.Scope
	config *config.S3StorageConfig
}

func newS3Service(cfg *config.S3StorageConfig, logger *zap.Logger, scope tally.Scope) (Service, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken)
		opts = append(opts, awsconfig.WithCredentialsProvider(creds))
	}

	// Configure HTTP client with SSL settings
	if cfg.UseSSL != nil && !*cfg.UseSSL {
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// #nosec G402 - Intentionally disabling TLS verification when UseSSL is false
					// This is for development/testing environments with self-signed certificates
					InsecureSkipVerify: true,
				},
			},
			Timeout: 30 * time.Second,
		}
		opts = append(opts, awsconfig.WithHTTPClient(httpClient))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	return &s3Service{
		client: client,
		logger: logger.Named("objectstorage"),
		scope:  scope.SubScope("objectstorage"),
		config: cfg,
	}, nil
}

func (s *s3Service) GetObject(ctx context.Context, bucket, path string) ([]byte, error) {
	if err := validateBucketName(bucket); err != nil {
		return nil, fmt.Errorf("invalid bucket: %w", err)
	}
	if err := validateObjectPath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	s.scope.Counter("get_object_requests").Inc(1)
	start := time.Now()
	defer func() {
		s.scope.Timer("get_object_duration").Record(time.Since(start))
	}()

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		s.scope.Counter("get_object_errors").Inc(1)
		s.logger.Error("failed to get object",
			zap.String("bucket", bucket),
			zap.String("path", sanitizePathForLogging(path)),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer func() { _ = out.Body.Close() }()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		s.scope.Counter("get_object_read_errors").Inc(1)
		return nil, fmt.Errorf("failed to read object data: %w", err)
	}

	s.scope.Counter("get_object_success").Inc(1)
	s.scope.Histogram("get_object_size_bytes", tally.DefaultBuckets).RecordValue(float64(len(data)))
	return data, nil
}

func (s *s3Service) PutObject(ctx context.Context, bucket, path string, content []byte) error {
	if err := validateBucketName(bucket); err != nil {
		return fmt.Errorf("invalid bucket: %w", err)
	}
	if err := validateObjectPath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if err := validateContentSize(content); err != nil {
		return fmt.Errorf("invalid content: %w", err)
	}

	s.scope.Counter("put_object_requests").Inc(1)
	s.scope.Histogram("put_object_size_bytes", tally.DefaultBuckets).RecordValue(float64(len(content)))
	start := time.Now()
	defer func() {
		s.scope.Timer("put_object_duration").Record(time.Since(start))
	}()

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(path),
		Body:        bytes.NewReader(content),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		s.scope.Counter("put_object_errors").Inc(1)
		s.logger.Error("failed to put object",
			zap.String("bucket", bucket),
			zap.String("path", sanitizePathForLogging(path)),
			zap.Int("size", len(content)),
			zap.Error(err))
		return fmt.Errorf("failed to put object: %w", err)
	}

	s.scope.Counter("put_object_success").Inc(1)
	return nil
}

func (s *s3Service) DeleteObject(ctx context.Context, bucket, path string) error {
	if err := validateBucketName(bucket); err != nil {
		return fmt.Errorf("invalid bucket: %w", err)
	}
	if err := validateObjectPath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	s.scope.Counter("delete_object_requests").Inc(1)
	start := time.Now()
	defer func() {
		s.scope.Timer("delete_object_duration").Record(time.Since(start))
	}()

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		s.scope.Counter("delete_object_errors").Inc(1)
		s.logger.Error("failed to delete object",
			zap.String("bucket", bucket),
			zap.String("path", sanitizePathForLogging(path)),
			zap.Error(err))
		return fmt.Errorf("failed to delete object: %w", err)
	}

	s.scope.Counter("delete_object_success").Inc(1)
	return nil
}

func (s *s3Service) ListObjects(ctx context.Context, bucket, prefix string) ([]Object, error) {
	if err := validateObjectPath(prefix); err != nil && prefix != "" {
		return nil, fmt.Errorf("invalid prefix: %w", err)
	}

	s.scope.Counter("list_objects_requests").Inc(1)
	start := time.Now()
	defer func() {
		s.scope.Timer("list_objects_duration").Record(time.Since(start))
	}()

	var results []Object
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			s.scope.Counter("list_objects_errors").Inc(1)
			s.logger.Error("failed to list objects",
				zap.String("bucket", bucket),
				zap.String("prefix", sanitizePathForLogging(prefix)),
				zap.Error(err))
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}
		for _, obj := range page.Contents {
			results = append(results, Object{
				Name:         aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified).String(),
			})
		}
	}

	s.scope.Counter("list_objects_success").Inc(1)
	s.scope.Histogram("list_objects_count", tally.DefaultBuckets).RecordValue(float64(len(results)))
	return results, nil
}

func (s *s3Service) Close() error {
	// S3 client doesn't need explicit closing
	return nil
}
