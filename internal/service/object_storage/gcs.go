package objectstorage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.admiral.io/admiral/internal/config"
)

type gcsService struct {
	client *storage.Client
	logger *zap.Logger
	scope  tally.Scope
	config *config.GCSStorageConfig
}

func newGCSService(cfg *config.GCSStorageConfig, logger *zap.Logger, scope tally.Scope) (Service, error) {
	var opts []option.ClientOption

	switch {
	case cfg.UseADC:
		// Application Default Credentials – no opts needed
	case cfg.CredentialsFile != "":
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	case cfg.CredentialsJSON != "":
		decoded, err := base64.StdEncoding.DecodeString(cfg.CredentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode GCS credentials_json: %w", err)
		}
		opts = append(opts, option.WithCredentialsJSON(decoded))
	default:
		logger.Warn("no GCS credentials specified; falling back to ADC")
	}

	client, err := storage.NewClient(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCS client: %w", err)
	}

	return &gcsService{
		client: client,
		logger: logger.Named("objectstorage"),
		scope:  scope.SubScope("objectstorage"),
		config: cfg,
	}, nil
}

func (s *gcsService) GetObject(ctx context.Context, bucket, path string) ([]byte, error) {
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

	reader, err := s.client.Bucket(bucket).Object(path).NewReader(ctx)
	if err != nil {
		s.scope.Counter("get_object_errors").Inc(1)
		s.logger.Error("failed to get object",
			zap.String("bucket", bucket),
			zap.String("path", sanitizePathForLogging(path)),
			zap.Error(err))
		return nil, fmt.Errorf("gcs: failed to read object: %w", err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		s.scope.Counter("get_object_read_errors").Inc(1)
		return nil, fmt.Errorf("failed to read object data: %w", err)
	}

	s.scope.Counter("get_object_success").Inc(1)
	s.scope.Histogram("get_object_size_bytes", tally.DefaultBuckets).RecordValue(float64(len(data)))
	return data, nil
}

func (s *gcsService) PutObject(ctx context.Context, bucket, path string, content []byte) error {
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

	writer := s.client.Bucket(bucket).Object(path).NewWriter(ctx)
	writer.ContentType = "application/octet-stream"

	if _, err := writer.Write(content); err != nil {
		_ = writer.Close()
		s.scope.Counter("put_object_errors").Inc(1)
		s.logger.Error("failed to write object",
			zap.String("bucket", bucket),
			zap.String("path", sanitizePathForLogging(path)),
			zap.Int("size", len(content)),
			zap.Error(err))
		return fmt.Errorf("gcs: failed to write object: %w", err)
	}

	if err := writer.Close(); err != nil {
		s.scope.Counter("put_object_errors").Inc(1)
		s.logger.Error("failed to finalize object",
			zap.String("bucket", bucket),
			zap.String("path", sanitizePathForLogging(path)),
			zap.Error(err))
		return fmt.Errorf("gcs: failed to finalize object: %w", err)
	}

	s.scope.Counter("put_object_success").Inc(1)
	return nil
}

func (s *gcsService) DeleteObject(ctx context.Context, bucket, path string) error {
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

	err := s.client.Bucket(bucket).Object(path).Delete(ctx)
	if err != nil {
		s.scope.Counter("delete_object_errors").Inc(1)
		s.logger.Error("failed to delete object",
			zap.String("bucket", bucket),
			zap.String("path", sanitizePathForLogging(path)),
			zap.Error(err))
		return fmt.Errorf("gcs: failed to delete object: %w", err)
	}

	s.scope.Counter("delete_object_success").Inc(1)
	return nil
}

func (s *gcsService) ListObjects(ctx context.Context, bucket, prefix string) ([]Object, error) {
	if err := validateObjectPath(prefix); err != nil && prefix != "" {
		return nil, fmt.Errorf("invalid prefix: %w", err)
	}

	s.scope.Counter("list_objects_requests").Inc(1)
	start := time.Now()
	defer func() {
		s.scope.Timer("list_objects_duration").Record(time.Since(start))
	}()

	var results []Object
	it := s.client.Bucket(bucket).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	for {
		attr, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			s.scope.Counter("list_objects_errors").Inc(1)
			s.logger.Error("failed to list objects",
				zap.String("bucket", bucket),
				zap.String("prefix", sanitizePathForLogging(prefix)),
				zap.Error(err))
			return nil, fmt.Errorf("gcs: list failed: %w", err)
		}
		results = append(results, Object{
			Name:         attr.Name,
			Size:         attr.Size,
			LastModified: attr.Updated.Format(time.RFC3339),
		})
	}

	s.scope.Counter("list_objects_success").Inc(1)
	s.scope.Histogram("list_objects_count", tally.DefaultBuckets).RecordValue(float64(len(results)))
	return results, nil
}

func (s *gcsService) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
