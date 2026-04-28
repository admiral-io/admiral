package objectstorage

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	scope := tally.NoopScope

	testCases := []struct {
		name        string
		config      *config.Config
		expected    string
		expectError bool
	}{
		{
			name:        "nil config",
			config:      &config.Config{}, // Empty config instead of nil
			expectError: true,
		},
		{
			name: "nil storage config",
			config: &config.Config{
				Services: config.Services{},
			},
			expectError: true,
		},
		{
			name: "valid S3 config",
			config: &config.Config{
				Services: config.Services{
					ObjectStorage: &config.ObjectStorage{
						Type: config.ObjectStorageTypeS3,
						S3: &config.S3StorageConfig{
							Region: "us-east-1",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid GCS config",
			config: &config.Config{
				Services: config.Services{
					ObjectStorage: &config.ObjectStorage{
						Type: config.ObjectStorageTypeGCS,
						GCS: &config.GCSStorageConfig{
							ProjectID: "test-project",
							UseADC:    true,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "S3 config without S3 details",
			config: &config.Config{
				Services: config.Services{
					ObjectStorage: &config.ObjectStorage{
						Type: config.ObjectStorageTypeS3,
					},
				},
			},
			expectError: true,
		},
		{
			name: "GCS config without GCS details",
			config: &config.Config{
				Services: config.Services{
					ObjectStorage: &config.ObjectStorage{
						Type: config.ObjectStorageTypeGCS,
					},
				},
			},
			expectError: true,
		},
		{
			name: "unsupported storage type",
			config: &config.Config{
				Services: config.Services{
					ObjectStorage: &config.ObjectStorage{
						Type: config.ObjectStorageType("invalid"),
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := New(tc.config, logger, scope)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, service)
				return
			}
			// GCS ADC initialization can fail in CI without credentials;
			// tolerate that specific failure mode so the test stays useful
			// while still catching genuine config bugs.
			if err != nil && isExpectedGCSAuthFailure(err) {
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, service)
		})
	}
}

// isExpectedGCSAuthFailure matches the error strings produced by GCS credential
// resolution paths that legitimately fail in CI environments without GCP auth
// configured. Used by tests to skip past auth-only failures while still
// catching configuration / wiring bugs.
func isExpectedGCSAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "failed to initialize GCS client") ||
		strings.Contains(msg, "detect GCS application default credentials") ||
		strings.Contains(msg, "load GCS service account credentials")
}

func TestValidateObjectPath(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
			errorMsg:    "path cannot be empty",
		},
		{
			name:        "valid simple path",
			path:        "folder/file.txt",
			expectError: false,
		},
		{
			name:        "valid path with underscores and numbers",
			path:        "folder_1/file_2.txt",
			expectError: false,
		},
		{
			name:        "path with parent directory traversal",
			path:        "../file.txt",
			expectError: true,
			errorMsg:    "invalid path: contains unsafe components",
		},
		{
			name:        "path with double dots",
			path:        "folder/../file.txt",
			expectError: true,
			errorMsg:    "invalid path: contains unsafe components",
		},
		{
			name:        "path starting with slash",
			path:        "/folder/file.txt",
			expectError: true,
			errorMsg:    "invalid path: contains unsafe components",
		},
		{
			name:        "path with double slashes",
			path:        "folder//file.txt",
			expectError: true,
			errorMsg:    "invalid path: contains unsafe components",
		},
		{
			name:        "path too long",
			path:        strings.Repeat("a", 1025),
			expectError: true,
			errorMsg:    "path too long: maximum 1024 characters allowed",
		},
		{
			name:        "path exactly at limit",
			path:        strings.Repeat("a", 1024),
			expectError: false,
		},
		{
			name:        "complex valid path",
			path:        "manifests/app-123/env-456/file.yaml",
			expectError: false,
		},
		{
			name:        "path with single dot",
			path:        "./file.txt",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateObjectPath(tc.path)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateBucketName(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty bucket name",
			bucket:      "",
			expectError: true,
			errorMsg:    "bucket name cannot be empty",
		},
		{
			name:        "valid bucket name",
			bucket:      "test-bucket",
			expectError: false,
		},
		{
			name:        "bucket name with numbers",
			bucket:      "bucket123",
			expectError: false,
		},
		{
			name:        "bucket name at max length",
			bucket:      strings.Repeat("a", 63),
			expectError: false,
		},
		{
			name:        "bucket name too long",
			bucket:      strings.Repeat("a", 64),
			expectError: true,
			errorMsg:    "bucket name too long: maximum 63 characters allowed",
		},
		{
			name:        "single character bucket",
			bucket:      "a",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBucketName(tc.bucket)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContentSize(t *testing.T) {
	testCases := []struct {
		name        string
		content     []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil content",
			content:     nil,
			expectError: true,
			errorMsg:    "content cannot be nil",
		},
		{
			name:        "empty content",
			content:     []byte{},
			expectError: false,
		},
		{
			name:        "small content",
			content:     []byte("hello world"),
			expectError: false,
		},
		{
			name:        "content at max size",
			content:     make([]byte, MaxObjectSize),
			expectError: false,
		},
		{
			name:        "content exceeding max size",
			content:     make([]byte, MaxObjectSize+1),
			expectError: true,
			errorMsg:    "content too large",
		},
		{
			name:        "large but valid content",
			content:     make([]byte, MaxObjectSize-1),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateContentSize(tc.content)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizePathForLogging(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "short path",
			path:     "short/path.txt",
			expected: "short/path.txt",
		},
		{
			name:     "path at 50 characters",
			path:     strings.Repeat("a", 50),
			expected: strings.Repeat("a", 50),
		},
		{
			name:     "long path gets truncated",
			path:     "this/is/a/very/long/path/that/should/be/truncated/for/security/reasons.txt",
			expected: "this/is/a/very/long/path/...r/security/reasons.txt",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "path exactly 51 characters",
			path:     strings.Repeat("a", 51),
			expected: strings.Repeat("a", 25) + "..." + strings.Repeat("a", 22),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizePathForLogging(tc.path)
			assert.Equal(t, tc.expected, result)

			// Ensure result is never longer than original if original was > 50
			if len(tc.path) > 50 {
				assert.LessOrEqual(t, len(result), len(tc.path))
			}
		})
	}
}

func TestConstants(t *testing.T) {
	t.Run("Name constant", func(t *testing.T) {
		assert.Equal(t, "service.objectstorage", Name)
	})

	t.Run("MaxObjectSize constant", func(t *testing.T) {
		expected := 50 * 1024 * 1024 // 50MB
		assert.Equal(t, expected, MaxObjectSize)
		assert.Equal(t, 52428800, MaxObjectSize) // 50MB in bytes
	})
}

func TestObject_Struct(t *testing.T) {
	t.Run("Object struct fields", func(t *testing.T) {
		obj := Object{
			Name:         "test-file.txt",
			Size:         1024,
			LastModified: "2023-01-01T00:00:00Z",
		}

		assert.Equal(t, "test-file.txt", obj.Name)
		assert.Equal(t, int64(1024), obj.Size)
		assert.Equal(t, "2023-01-01T00:00:00Z", obj.LastModified)
	})

	t.Run("zero value Object", func(t *testing.T) {
		var obj Object
		assert.Equal(t, "", obj.Name)
		assert.Equal(t, int64(0), obj.Size)
		assert.Equal(t, "", obj.LastModified)
	})
}

// Test the Service interface implementation.
func TestService_Interface(t *testing.T) {
	t.Run("Service interface methods", func(t *testing.T) {
		// This test ensures that any implementation of Service has the correct methods
		var _ Service = (*mockService)(nil)
	})
}

// mockService is a minimal implementation to test interface compliance.
type mockService struct{}

func (m *mockService) ListObjects(ctx context.Context, bucket, prefix string) ([]Object, error) {
	return nil, nil
}

func (m *mockService) GetObject(ctx context.Context, bucket, path string) ([]byte, error) {
	return nil, nil
}

func (m *mockService) PutObject(ctx context.Context, bucket, path string, content []byte) error {
	return nil
}

func (m *mockService) DeleteObject(ctx context.Context, bucket, path string) error {
	return nil
}

func (m *mockService) Close() error {
	return nil
}

func TestNew_EdgeCases(t *testing.T) {
	logger := zap.NewNop()
	scope := tally.NoopScope

	t.Run("valid S3 config", func(t *testing.T) {
		cfg := &config.Config{
			Services: config.Services{
				ObjectStorage: &config.ObjectStorage{
					Type: config.ObjectStorageTypeS3,
					S3: &config.S3StorageConfig{
						Region: "us-east-1",
					},
				},
			},
		}

		service, err := New(cfg, logger, scope)
		assert.NoError(t, err)
		assert.NotNil(t, service)

		// Clean up
		if closer, ok := service.(Service); ok {
			err = closer.Close()
			assert.NoError(t, err)
		}
	})

	t.Run("valid GCS config", func(t *testing.T) {
		cfg := &config.Config{
			Services: config.Services{
				ObjectStorage: &config.ObjectStorage{
					Type: config.ObjectStorageTypeGCS,
					GCS: &config.GCSStorageConfig{
						ProjectID: "test-project",
						UseADC:    true,
					},
				},
			},
		}

		// This may fail due to GCS client initialization in test environment
		service, err := New(cfg, logger, scope)
		if err != nil && !strings.Contains(err.Error(), "failed to initialize GCS client") {
			t.Errorf("Configuration should be valid: %v", err)
		}
		if service != nil {
			if closer, ok := service.(Service); ok {
				_ = closer.Close()
			}
		}
	})

	t.Run("config with services but no storage", func(t *testing.T) {
		cfg := &config.Config{
			Services: config.Services{
				// Storage is nil
			},
		}

		service, err := New(cfg, logger, scope)
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "storage configuration is required")
	})
}

func TestValidation_SecurityFocus(t *testing.T) {
	t.Run("validateObjectPath security tests", func(t *testing.T) {
		maliciousPaths := []string{
			"../../etc/passwd",
			"../../../root/.ssh/id_rsa",
			"/etc/passwd",
			"folder/../../sensitive",
			"folder//file",
			"./../config",
		}

		for _, path := range maliciousPaths {
			t.Run("malicious path: "+path, func(t *testing.T) {
				err := validateObjectPath(path)
				assert.Error(t, err, "Should reject malicious path: %s", path)
			})
		}
	})

	t.Run("validateContentSize DoS protection", func(t *testing.T) {
		// Test exactly at the boundary
		maxContent := make([]byte, MaxObjectSize)
		err := validateContentSize(maxContent)
		assert.NoError(t, err)

		// Test just over the boundary
		overSizeContent := make([]byte, MaxObjectSize+1)
		err = validateContentSize(overSizeContent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content too large")
	})
}
