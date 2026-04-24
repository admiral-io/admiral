package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectStorageTypeValidate(t *testing.T) {
	tests := []struct {
		t       ObjectStorageType
		wantErr bool
	}{
		{ObjectStorageTypeS3, false},
		{ObjectStorageTypeGCS, false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.t), func(t *testing.T) {
			err := tt.t.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestObjectStorageTypeString(t *testing.T) {
	s3 := ObjectStorageTypeS3
	assert.Equal(t, "s3", s3.String())

	var nilType *ObjectStorageType
	assert.Equal(t, "unspecified", nilType.String())
}

func TestObjectStorageValidate(t *testing.T) {
	tests := []struct {
		name    string
		os      *ObjectStorage
		wantErr string
	}{
		{name: "nil", os: nil},
		{name: "valid s3", os: &ObjectStorage{Type: ObjectStorageTypeS3, Bucket: "b", S3: &S3StorageConfig{}}},
		{name: "valid gcs", os: &ObjectStorage{Type: ObjectStorageTypeGCS, Bucket: "b", GCS: &GCSStorageConfig{}}},
		{name: "missing type", os: &ObjectStorage{Bucket: "b"}, wantErr: "type is required"},
		{name: "missing bucket", os: &ObjectStorage{Type: ObjectStorageTypeS3}, wantErr: "bucket is required"},
		{name: "s3 without config", os: &ObjectStorage{Type: ObjectStorageTypeS3, Bucket: "b"}, wantErr: "S3 config is required"},
		{name: "gcs without config", os: &ObjectStorage{Type: ObjectStorageTypeGCS, Bucket: "b"}, wantErr: "GCS config is required"},
		{name: "unsupported type", os: &ObjectStorage{Type: "azure", Bucket: "b"}, wantErr: "unsupported storage type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.os.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestObjectStorageSetDefaults_S3(t *testing.T) {
	os := &ObjectStorage{
		Type: ObjectStorageTypeS3,
		S3:   &S3StorageConfig{},
	}
	os.SetDefaults()

	require.NotNil(t, os.S3.UseSSL)
	assert.True(t, *os.S3.UseSSL)
}

func TestObjectStorageSetDefaults_NilReceiver(t *testing.T) {
	var os *ObjectStorage
	os.SetDefaults() // should not panic
}
