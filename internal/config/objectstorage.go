package config

import "fmt"

type ObjectStorageType string

const (
	ObjectStorageTypeS3  ObjectStorageType = "s3"
	ObjectStorageTypeGCS ObjectStorageType = "gcs"
)

func (s *ObjectStorageType) String() string {
	if s == nil {
		return "unspecified"
	}

	return string(*s)
}

func (s *ObjectStorageType) Validate() error {
	switch *s {
	case ObjectStorageTypeS3, ObjectStorageTypeGCS:
		return nil
	default:
		return fmt.Errorf("invalid object storage type: %q", *s)
	}
}

type S3StorageConfig struct {
	Endpoint     string `yaml:"endpoint"`
	Region       string `yaml:"region"`
	UseSSL       *bool  `yaml:"use_ssl"`
	AccessKey    string `yaml:"access_key"`
	SecretKey    string `yaml:"secret_key"`
	RoleARN      string `yaml:"role_arn"`
	SessionToken string `yaml:"session_token"`
}

func (s *S3StorageConfig) SetDefaults() {
	if s == nil {
		return
	}

	if s.UseSSL == nil {
		useSSL := true
		s.UseSSL = &useSSL
	}
}

func (s *S3StorageConfig) Validate() error {
	if s == nil {
		return nil
	}

	return nil
}

type GCSStorageConfig struct {
	ProjectID       string `yaml:"project_id"`
	CredentialsFile string `yaml:"credentials_file"`
	CredentialsJSON string `yaml:"credentials_json"`
	UseADC          bool   `yaml:"use_adc"`
}

func (g *GCSStorageConfig) SetDefaults() {
	if g == nil {
		return
	}
}

func (g *GCSStorageConfig) Validate() error {
	if g == nil {
		return nil
	}

	count := 0
	if g.UseADC {
		count++
	}
	if g.CredentialsFile != "" {
		count++
	}
	if g.CredentialsJSON != "" {
		count++
	}
	if count == 0 {
		return fmt.Errorf("one of use_adc, credentials_file, or credentials_json must be set")
	}
	if count > 1 {
		return fmt.Errorf("use_adc, credentials_file, and credentials_json are mutually exclusive")
	}
	return nil
}

type ObjectStorage struct {
	Type   ObjectStorageType `yaml:"type"`
	Bucket string            `yaml:"bucket"`
	S3     *S3StorageConfig  `yaml:"s3,omitempty"`
	GCS    *GCSStorageConfig `yaml:"gcs,omitempty"`
}

func (s *ObjectStorage) SetDefaults() {
	if s == nil {
		return
	}

	if s.Type == ObjectStorageTypeS3 && s.S3 != nil {
		s.S3.SetDefaults()
	}

	if s.Type == ObjectStorageTypeGCS && s.GCS != nil {
		s.GCS.SetDefaults()
	}
}

func (s *ObjectStorage) Validate() error {
	if s == nil {
		return nil
	}

	if s.Type == "" {
		return fmt.Errorf("type is required")
	}
	if s.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}

	switch s.Type {
	case ObjectStorageTypeS3:
		if s.S3 == nil {
			return fmt.Errorf("S3 config is required for type %q", s.Type)
		}

		return s.S3.Validate()
	case ObjectStorageTypeGCS:
		if s.GCS == nil {
			return fmt.Errorf("GCS config is required for type %q", s.Type)
		}

		return s.GCS.Validate()
	default:
		return fmt.Errorf("unsupported storage type: %q", s.Type)
	}
}
