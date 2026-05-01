package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type AccessTokenStore struct {
	db *gorm.DB
}

func NewAccessTokenStore(db *gorm.DB) (*AccessTokenStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &AccessTokenStore{db: db}, nil
}

func (s *AccessTokenStore) DB() *gorm.DB {
	return s.db
}

func (s *AccessTokenStore) Create(ctx context.Context, token *model.AccessToken) (*model.AccessToken, error) {
	if err := token.Validate(); err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}
	if err := s.db.WithContext(ctx).Create(token).Error; err != nil {
		return nil, fmt.Errorf("failed to create access token: %w", err)
	}
	return token, nil
}

// Get returns the access token with the given id regardless of status.
// Callers that need to enforce active-only semantics (e.g. authentication)
// must check token.Status themselves.
func (s *AccessTokenStore) Get(ctx context.Context, id string) (*model.AccessToken, error) {
	var token model.AccessToken
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("access token not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access token with id %s: %w", id, err)
	}
	return &token, nil
}

// GetByHash returns the access token with the given hash regardless of
// status. Callers in the auth path must check token.Status (only ACTIVE
// tokens may authenticate).
func (s *AccessTokenStore) GetByHash(ctx context.Context, hash []byte) (*model.AccessToken, error) {
	var token model.AccessToken
	err := s.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("access token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access token by hash: %w", err)
	}
	return &token, nil
}

func (s *AccessTokenStore) ListBySubject(ctx context.Context, subject string, kind string) ([]model.AccessToken, error) {
	query := s.db.WithContext(ctx).Where("subject = ?", subject)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	var tokens []model.AccessToken
	err := query.Order("created_at DESC").Find(&tokens).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list access tokens for subject %s: %w", subject, err)
	}
	return tokens, nil
}

func (s *AccessTokenStore) UpdateIdPTokens(ctx context.Context, id string, idpToken *oauth2.Token, expiresAt *time.Time) error {
	updates := map[string]any{
		"expires_at": expiresAt,
		"updated_at": time.Now(),
	}
	if idpToken != nil {
		if idpToken.AccessToken != "" {
			updates["idp_access_token"] = []byte(idpToken.AccessToken)
		}
		if idpToken.RefreshToken != "" {
			updates["idp_refresh_token"] = []byte(idpToken.RefreshToken)
		}
		if it, ok := idpToken.Extra("id_token").(string); ok && it != "" {
			updates["idp_id_token"] = []byte(it)
		}
	}
	result := s.db.WithContext(ctx).
		Model(&model.AccessToken{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update IdP tokens: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("access token not found: %s", id)
	}
	return nil
}

func (s *AccessTokenStore) Update(ctx context.Context, id string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AccessToken{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update access token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("access token not found: %s", id)
	}
	return nil
}

func (s *AccessTokenStore) Revoke(ctx context.Context, id string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AccessToken{}).
		Where("id = ? AND deleted_at IS NULL AND status = ?", id, model.AccessTokenStatusActive).
		Updates(map[string]any{
			"status":     model.AccessTokenStatusRevoked,
			"revoked_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke access token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("no active access token found to revoke")
	}
	return nil
}

// Delete soft-deletes the access token and flips its status to REVOKED so
// it cannot be used even if the soft-delete is later reversed. Idempotent:
// no error if no matching token is found, matching DeleteBySubject.
func (s *AccessTokenStore) Delete(ctx context.Context, id string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AccessToken{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"status":     model.AccessTokenStatusRevoked,
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to delete access token: %w", result.Error)
	}
	return nil
}

func (s *AccessTokenStore) DeleteBySubject(ctx context.Context, subject string) (int64, error) {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AccessToken{}).
		Where("subject = ? AND deleted_at IS NULL", subject).
		Updates(map[string]any{
			"status":     model.AccessTokenStatusRevoked,
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to revoke access tokens for subject %s: %w", subject, result.Error)
	}
	return result.RowsAffected, nil
}
