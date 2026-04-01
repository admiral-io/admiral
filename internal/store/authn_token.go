package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type AuthnTokenStore struct {
	db *gorm.DB
}

func NewAuthnTokenStore(db *gorm.DB) (*AuthnTokenStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &AuthnTokenStore{db: db}, nil
}

func (s *AuthnTokenStore) Save(ctx context.Context, tokenId uuid.UUID, parentTokenId *uuid.UUID, subject string, issuer string, kind model.AuthnTokenKind, token *oauth2.Token) (*model.AuthnToken, error) {
	if tokenId == uuid.Nil {
		return nil, errors.New("id cannot be empty")
	}

	if subject == "" {
		return nil, errors.New("subject cannot be empty")
	}

	if issuer == "" {
		return nil, errors.New("issuer cannot be empty")
	}

	if token == nil {
		return nil, errors.New("token provided for storage was nil")
	}

	if token.AccessToken == "" {
		return nil, errors.New("access token cannot be empty")
	}

	if token.Expiry.IsZero() || token.Expiry.Before(time.Now()) {
		return nil, errors.New("token expiry is invalid")
	}

	var existing model.AuthnToken
	err := s.db.WithContext(ctx).First(&existing, "id = ?", tokenId).Error

	if err == nil {
		updates := map[string]any{
			"subject":      subject,
			"issuer":       issuer,
			"kind":         kind,
			"access_token": []byte(token.AccessToken),
			"expires_at":   token.Expiry,
			"updated_at":   time.Now(),
		}

		if parentTokenId != nil {
			updates["parent_id"] = *parentTokenId
		}

		if token.RefreshToken != "" {
			updates["refresh_token"] = []byte(token.RefreshToken)
		}

		if it, ok := token.Extra("id_token").(string); ok && it != "" {
			updates["id_token"] = []byte(it)
		}

		err = s.db.WithContext(ctx).Model(&existing).Updates(updates).Error
		if err != nil {
			return nil, fmt.Errorf("failed to update token: %w", err)
		}

		err = s.db.WithContext(ctx).First(&existing, "id = ?", tokenId).Error
		if err != nil {
			return nil, fmt.Errorf("failed to reload updated token: %w", err)
		}

		return &existing, nil
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		authnToken := &model.AuthnToken{
			Id:          tokenId,
			ParentID:    parentTokenId,
			Subject:     subject,
			Issuer:      issuer,
			Kind:        kind,
			AccessToken: []byte(token.AccessToken),
			ExpiresAt:   token.Expiry,
		}

		if token.RefreshToken != "" {
			authnToken.RefreshToken = []byte(token.RefreshToken)
		}

		if it, ok := token.Extra("id_token").(string); ok && it != "" {
			authnToken.IdToken = []byte(it)
		}

		err = s.db.WithContext(ctx).Create(authnToken).Error
		if err != nil {
			return nil, fmt.Errorf("failed to create token: %w", err)
		}

		return authnToken, nil
	}

	return nil, fmt.Errorf("failed to check for existing token: %w", err)
}

func (s *AuthnTokenStore) Get(ctx context.Context, id uuid.UUID) (*model.AuthnToken, error) {
	if id == uuid.Nil {
		return nil, errors.New("id cannot be empty")
	}

	var authnToken model.AuthnToken
	err := s.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, model.AuthnTokenStatusActive).
		First(&authnToken).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("token not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve authn token with id %s: %w", id, err)
	}

	return &authnToken, nil
}

func (s *AuthnTokenStore) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("id cannot be empty")
	}

	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AuthnToken{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"status":     model.AuthnTokenStatusRevoked,
			"deleted_at": now,
			"updated_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to revoke authn token: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("no token found to revoke")
	}

	return nil
}

func (s *AuthnTokenStore) DeleteBySubject(ctx context.Context, subject string) (int64, error) {
	if subject == "" {
		return 0, errors.New("subject cannot be empty")
	}

	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AuthnToken{}).
		Where("subject = ? AND deleted_at IS NULL", subject).
		Updates(map[string]any{
			"status":     model.AuthnTokenStatusRevoked,
			"deleted_at": now,
			"updated_at": now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to revoke tokens for subject %s: %w", subject, result.Error)
	}

	return result.RowsAffected, nil
}
