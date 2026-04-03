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

type AuthSessionStore struct {
	db *gorm.DB
}

func NewAuthSessionStore(db *gorm.DB) (*AuthSessionStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &AuthSessionStore{db: db}, nil
}

func (s *AuthSessionStore) Save(ctx context.Context, id uuid.UUID, parentID *uuid.UUID, subject string, issuer string, token *oauth2.Token) (*model.AuthSession, error) {
	if id == uuid.Nil {
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

	if !token.Expiry.IsZero() && token.Expiry.Before(time.Now()) {
		return nil, errors.New("token expiry is invalid")
	}

	var existing model.AuthSession
	err := s.db.WithContext(ctx).First(&existing, "id = ?", id).Error

	if err == nil {
		updates := map[string]any{
			"subject":      subject,
			"issuer":       issuer,
			"access_token": []byte(token.AccessToken),
			"expires_at":   timeToPtr(token.Expiry),
			"updated_at":   time.Now(),
		}

		if parentID != nil {
			updates["parent_id"] = *parentID
		}

		if token.RefreshToken != "" {
			updates["refresh_token"] = []byte(token.RefreshToken)
		}

		if it, ok := token.Extra("id_token").(string); ok && it != "" {
			updates["id_token"] = []byte(it)
		}

		err = s.db.WithContext(ctx).Model(&existing).Updates(updates).Error
		if err != nil {
			return nil, fmt.Errorf("failed to update session: %w", err)
		}

		err = s.db.WithContext(ctx).First(&existing, "id = ?", id).Error
		if err != nil {
			return nil, fmt.Errorf("failed to reload updated session: %w", err)
		}

		return &existing, nil
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		session := &model.AuthSession{
			Id:          id,
			ParentID:    parentID,
			Subject:     subject,
			Issuer:      issuer,
			AccessToken: []byte(token.AccessToken),
			ExpiresAt:   timeToPtr(token.Expiry),
		}

		if token.RefreshToken != "" {
			session.RefreshToken = []byte(token.RefreshToken)
		}

		if it, ok := token.Extra("id_token").(string); ok && it != "" {
			session.IdToken = []byte(it)
		}

		err = s.db.WithContext(ctx).Create(session).Error
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}

		return session, nil
	}

	return nil, fmt.Errorf("failed to check for existing session: %w", err)
}

func (s *AuthSessionStore) Get(ctx context.Context, id uuid.UUID) (*model.AuthSession, error) {
	if id == uuid.Nil {
		return nil, errors.New("id cannot be empty")
	}

	var session model.AuthSession
	err := s.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, model.AuthSessionStatusActive).
		First(&session).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve session with id %s: %w", id, err)
	}

	return &session, nil
}

func (s *AuthSessionStore) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("id cannot be empty")
	}

	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AuthSession{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"status":     model.AuthSessionStatusRevoked,
			"deleted_at": now,
			"updated_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to revoke session: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("no session found to revoke")
	}

	return nil
}

func (s *AuthSessionStore) DeleteBySubject(ctx context.Context, subject string) (int64, error) {
	if subject == "" {
		return 0, errors.New("subject cannot be empty")
	}

	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&model.AuthSession{}).
		Where("subject = ? AND deleted_at IS NULL", subject).
		Updates(map[string]any{
			"status":     model.AuthSessionStatusRevoked,
			"deleted_at": now,
			"updated_at": now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to revoke sessions for subject %s: %w", subject, result.Error)
	}

	return result.RowsAffected, nil
}
