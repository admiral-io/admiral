package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
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
	if token == nil {
		return nil, errors.New("token cannot be nil")
	}

	if token.Subject == "" {
		return nil, errors.New("subject cannot be empty")
	}

	if len(token.TokenHash) == 0 {
		return nil, errors.New("token hash cannot be empty")
	}

	err := s.db.WithContext(ctx).Create(token).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create access token: %w", err)
	}

	return token, nil
}

func (s *AccessTokenStore) Get(ctx context.Context, id string) (*model.AccessToken, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	var token model.AccessToken
	err := s.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, model.AccessTokenStatusActive).
		First(&token).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("access token not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access token with id %s: %w", id, err)
	}

	return &token, nil
}

func (s *AccessTokenStore) GetByHash(ctx context.Context, hash []byte) (*model.AccessToken, error) {
	if len(hash) == 0 {
		return nil, errors.New("token hash cannot be empty")
	}

	var token model.AccessToken
	err := s.db.WithContext(ctx).
		Where("token_hash = ? AND status = ?", hash, model.AccessTokenStatusActive).
		First(&token).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("access token not found")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access token by hash: %w", err)
	}

	return &token, nil
}

func (s *AccessTokenStore) ListBySubject(ctx context.Context, subject string, kind string) ([]model.AccessToken, error) {
	if subject == "" {
		return nil, errors.New("subject cannot be empty")
	}

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
	if id == "" {
		return errors.New("id cannot be empty")
	}

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
		Where("id = ? AND status = ?", id, model.AccessTokenStatusActive).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update IdP tokens: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("no active access token found to update")
	}

	return nil
}

func (s *AccessTokenStore) UpdateScopes(ctx context.Context, id string, scopes []string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	result := s.db.WithContext(ctx).
		Model(&model.AccessToken{}).
		Where("id = ? AND status = ?", id, model.AccessTokenStatusActive).
		Updates(map[string]any{
			"scopes":     pq.StringArray(scopes),
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update scopes: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("no active access token found to update")
	}

	return nil
}

func (s *AccessTokenStore) Update(ctx context.Context, id string, updates map[string]any) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}
	if len(updates) == 0 {
		return nil
	}

	updates["updated_at"] = time.Now()

	result := s.db.WithContext(ctx).
		Model(&model.AccessToken{}).
		Where("id = ? AND status = ?", id, model.AccessTokenStatusActive).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update access token: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("no active access token found to update")
	}

	return nil
}

func (s *AccessTokenStore) Revoke(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

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

func (s *AccessTokenStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

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

	if result.RowsAffected == 0 {
		return errors.New("no access token found to delete")
	}

	return nil
}

func (s *AccessTokenStore) DeleteBySubject(ctx context.Context, subject string) (int64, error) {
	if subject == "" {
		return 0, errors.New("subject cannot be empty")
	}

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
