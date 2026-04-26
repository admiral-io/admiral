package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/crypto"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service/encryption"
)

// ErrInvalidAuthConfig indicates the credential's AuthConfig does not match
// its declared type or is missing required fields. Callers should map this to
// a client-facing validation error (e.g. gRPC InvalidArgument).
var ErrInvalidAuthConfig = errors.New("invalid auth config")

type CredentialStore struct {
	db  *gorm.DB
	enc encryption.Service
}

func NewCredentialStore(db *gorm.DB, enc encryption.Service) (*CredentialStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	if enc == nil {
		return nil, errors.New("encryptor is required")
	}

	return &CredentialStore{db: db, enc: enc}, nil
}

func (s *CredentialStore) DB() *gorm.DB {
	return s.db
}

func (s *CredentialStore) Create(ctx context.Context, cred *model.Credential) (*model.Credential, error) {
	if err := cred.AuthConfig.Validate(cred.Type); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAuthConfig, err)
	}

	if err := s.encryptAuthConfig(cred); err != nil {
		return nil, fmt.Errorf("failed to encrypt auth config: %w", err)
	}

	if err := s.db.WithContext(ctx).Create(cred).Error; err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	if err := s.decryptAuthConfig(cred); err != nil {
		return nil, fmt.Errorf("failed to decrypt auth config: %w", err)
	}

	return cred, nil
}

func (s *CredentialStore) Get(ctx context.Context, id uuid.UUID) (*model.Credential, error) {
	var cred model.Credential
	err := s.db.WithContext(ctx).Scopes(WithActorRef("credentials", "created_by")).Where("credentials.id = ?", id).Take(&cred).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("credential not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	if err := s.decryptAuthConfig(&cred); err != nil {
		return nil, fmt.Errorf("failed to decrypt auth config: %w", err)
	}

	return &cred, nil
}

func (s *CredentialStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Credential, error) {
	var creds []model.Credential
	err := s.db.WithContext(ctx).Scopes(append(scopes, WithActorRef("credentials", "created_by"))...).Find(&creds).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	for i := range creds {
		if err := s.decryptAuthConfig(&creds[i]); err != nil {
			return nil, fmt.Errorf("failed to decrypt auth config for credential %s: %w", creds[i].Id, err)
		}
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
		encrypted, err := s.encryptAuthConfigValue(auth)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt auth config: %w", err)
		}
		fields["auth_config"] = encrypted
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

// encryptAuthConfig encrypts the AuthConfig on a credential in-place before
// a GORM Create. The AuthConfig is marshalled to JSON, encrypted, and the
// result is stored back so that Value() writes the envelope to the DB.
// Empty auth configs (Kind == "") are left as-is.
func (s *CredentialStore) encryptAuthConfig(cred *model.Credential) error {
	if cred.AuthConfig.Kind == "" {
		return nil
	}

	plaintext, err := json.Marshal(cred.AuthConfig)
	if err != nil {
		return fmt.Errorf("marshalling auth config: %w", err)
	}

	envelope, err := s.enc.Encrypt(plaintext)
	if err != nil {
		return err
	}

	// Replace the AuthConfig with a raw envelope that Value() will pass through.
	cred.AuthConfig = model.AuthConfig{}
	cred.AuthConfig.SetRaw(envelope)
	return nil
}

// encryptAuthConfigValue encrypts an AuthConfig value for use in Update field maps.
func (s *CredentialStore) encryptAuthConfigValue(ac model.AuthConfig) (model.AuthConfig, error) {
	if ac.Kind == "" {
		return ac, nil
	}

	plaintext, err := json.Marshal(ac)
	if err != nil {
		return model.AuthConfig{}, fmt.Errorf("marshalling auth config: %w", err)
	}

	envelope, err := s.enc.Encrypt(plaintext)
	if err != nil {
		return model.AuthConfig{}, err
	}

	var encrypted model.AuthConfig
	encrypted.SetRaw(envelope)
	return encrypted, nil
}

// decryptAuthConfig decrypts the AuthConfig on a credential in-place after
// a GORM read. Empty or non-envelope values are left as-is.
func (s *CredentialStore) decryptAuthConfig(cred *model.Credential) error {
	raw := cred.AuthConfig.Raw()
	if raw == nil || !crypto.IsEnvelope(raw) {
		return nil
	}

	plaintext, err := s.enc.DecryptAny(raw)
	if err != nil {
		return err
	}

	return json.Unmarshal(plaintext, &cred.AuthConfig)
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
