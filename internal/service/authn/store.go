package authn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/model"
)

type store struct {
	database *gorm.DB
}

func newStore(_ *config.Config, db *gorm.DB) (*store, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &store{
		database: db,
	}, nil
}

func (s *store) save(ctx context.Context, tokenId uuid.UUID, parentTokenId *uuid.UUID, subject string, issuer string, kind model.AuthnTokenKind, token *oauth2.Token) (*model.AuthnToken, error) {
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

	// Check if token with this ID already exists
	var existing model.AuthnToken
	err := s.database.WithContext(ctx).First(&existing, "id = ?", tokenId).Error

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

		err = s.database.WithContext(ctx).Model(&existing).Updates(updates).Error
		if err != nil {
			return nil, fmt.Errorf("failed to update token: %w", err)
		}

		// Reload the updated token
		err = s.database.WithContext(ctx).First(&existing, "id = ?", tokenId).Error
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

		err = s.database.WithContext(ctx).Create(authnToken).Error
		if err != nil {
			return nil, fmt.Errorf("failed to create token: %w", err)
		}

		return authnToken, nil
	} else {
		return nil, fmt.Errorf("failed to check for existing token: %w", err)
	}
}

func (s *store) get(ctx context.Context, id uuid.UUID) (*model.AuthnToken, error) {
	if id == uuid.Nil {
		return nil, errors.New("id cannot be empty")
	}

	var authnToken model.AuthnToken
	err := s.database.WithContext(ctx).
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

func (s *store) delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("id cannot be empty")
	}

	now := time.Now()
	result := s.database.WithContext(ctx).
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

func (s *store) upsertUserFromClaims(ctx context.Context, claims *Claims) (*model.User, error) {
	if claims == nil || claims.Subject == "" {
		return nil, errors.New("invalid claims: nil, or missing subject")
	}

	var user model.User
	result := s.database.WithContext(ctx).Where("deleted_at IS NULL").First(&user, "provider_subject = ?", claims.Subject)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to retrieve user for subject %s: %v", claims.Subject, result.Error)
		}

		user = model.User{
			ProviderSubject: claims.Subject,
			Email:           claims.Email,
			EmailVerified:   claims.EmailVerified,
			Name:            claims.Name,
			GivenName:       claims.GivenName,
			FamilyName:      claims.FamilyName,
			PictureUrl:      claims.Picture,
		}
		if err := s.database.WithContext(ctx).Create(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user for subject %s: %v", claims.Subject, err)
		}
		return &user, nil
	} else {
		user.Email = claims.Email
		user.EmailVerified = claims.EmailVerified
		user.Name = claims.Name
		user.GivenName = claims.GivenName
		user.FamilyName = claims.FamilyName
		user.PictureUrl = claims.Picture

		if err := s.database.WithContext(ctx).Save(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to update user for subject %s: %v", claims.Subject, err)
		}
	}

	return &user, nil
}
