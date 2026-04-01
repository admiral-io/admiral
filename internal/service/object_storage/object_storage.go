package objectstorage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/service"
)

const (
	Name          = "service.objectstorage"
	MaxObjectSize = 50 * 1024 * 1024 // 50MB
)

type Service interface {
	ListObjects(ctx context.Context, bucket, prefix string) ([]Object, error)
	GetObject(ctx context.Context, bucket, path string) ([]byte, error)
	PutObject(ctx context.Context, bucket, path string, content []byte) error
	DeleteObject(ctx context.Context, bucket, path string) error
	io.Closer
}

type Object struct {
	Name         string
	Size         int64
	LastModified string
}

func New(cfg *config.Config, logger *zap.Logger, scope tally.Scope) (service.Service, error) {
	if cfg.Services.ObjectStorage == nil {
		return nil, fmt.Errorf("storage configuration is required")
	}

	switch cfg.Services.ObjectStorage.Type {
	case config.ObjectStorageTypeS3:
		if cfg.Services.ObjectStorage.S3 == nil {
			return nil, fmt.Errorf("S3 configuration is required for storage type %q", cfg.Services.ObjectStorage.Type)
		}
		return newS3Service(cfg.Services.ObjectStorage.S3, logger, scope)
	case config.ObjectStorageTypeGCS:
		if cfg.Services.ObjectStorage.GCS == nil {
			return nil, fmt.Errorf("GCS configuration is required for storage type %q", cfg.Services.ObjectStorage.Type)
		}
		return newGCSService(cfg.Services.ObjectStorage.GCS, logger, scope)
	default:
		return nil, fmt.Errorf("unsupported storage type: %q", cfg.Services.ObjectStorage.Type)
	}
}

// validateObjectPath sanitizes and validates object paths to prevent directory traversal
func validateObjectPath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// First check the original path for suspicious patterns
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || strings.Contains(path, "//") {
		return fmt.Errorf("invalid path: contains unsafe components")
	}

	// Clean the path to remove any . components and normalize
	clean := filepath.Clean(path)

	// Additional validation after cleaning
	if strings.HasPrefix(clean, "/") {
		return fmt.Errorf("invalid path: absolute paths not allowed")
	}

	// Additional validation - reject very long paths
	if len(clean) > 1024 {
		return fmt.Errorf("path too long: maximum 1024 characters allowed")
	}

	return nil
}

// validateBucketName validates bucket names
func validateBucketName(bucket string) error {
	if bucket == "" {
		return fmt.Errorf("bucket name cannot be empty")
	}
	if len(bucket) > 63 {
		return fmt.Errorf("bucket name too long: maximum 63 characters allowed")
	}
	return nil
}

// validateContentSize validates content size limits
func validateContentSize(content []byte) error {
	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}
	if len(content) > MaxObjectSize {
		return fmt.Errorf("content too large: maximum %d bytes allowed", MaxObjectSize)
	}
	return nil
}

// sanitizePathForLogging removes sensitive information from paths for logging
func sanitizePathForLogging(path string) string {
	if len(path) > 50 {
		return path[:25] + "..." + path[len(path)-22:]
	}
	return path
}
