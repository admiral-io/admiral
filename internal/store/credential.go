package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

// ErrInvalidAuthConfig indicates the credential's AuthConfig does not match
// its declared type or is missing required fields. Callers should map this to
// a client-facing validation error (e.g. gRPC InvalidArgument).
var ErrInvalidAuthConfig = errors.New("invalid auth config")

type CredentialStore struct {
	db *gorm.DB
}

func NewCredentialStore(db *gorm.DB) (*CredentialStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &CredentialStore{db: db}, nil
}

func (s *CredentialStore) DB() *gorm.DB {
	return s.db
}

func (s *CredentialStore) Create(ctx context.Context, cred *model.Credential) (*model.Credential, error) {
	if err := cred.AuthConfig.Validate(cred.Type); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAuthConfig, err)
	}

	if err := s.db.WithContext(ctx).Create(cred).Error; err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	return cred, nil
}

func (s *CredentialStore) Get(ctx context.Context, id uuid.UUID) (*model.Credential, error) {
	var cred model.Credential
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&cred).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("credential not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	return &cred, nil
}

func (s *CredentialStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Credential, error) {
	var creds []model.Credential
	err := s.db.WithContext(ctx).Scopes(scopes...).Find(&creds).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	return creds, nil
}

func (s *CredentialStore) Update(ctx context.Context, cred *model.Credential, fields map[string]any) (*model.Credential, error) {
	if ac, ok := fields["auth_config"]; ok {
		auth, ok := ac.(model.AuthConfig)
		if !ok {
			return nil, fmt.Errorf("%w: auth_config must be model.AuthConfig, got %T", ErrInvalidAuthConfig, ac)
		}
		if err := auth.Validate(cred.Type); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidAuthConfig, err)
		}
	}

	result := s.db.WithContext(ctx).Model(cred).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update credential: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("credential not found: %s", cred.Id)
	}

	return s.Get(ctx, cred.Id)
}

func (s *CredentialStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Credential{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete credential: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("credential not found: %s", id)
	}

	return nil
}
