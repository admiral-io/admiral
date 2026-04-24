package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type UserStore struct {
	db *gorm.DB
}

func NewUserStore(db *gorm.DB) (*UserStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &UserStore{db: db}, nil
}

func (s *UserStore) DB() *gorm.DB {
	return s.db
}

func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("user not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (s *UserStore) UpsertByProviderSubject(ctx context.Context, providerSubject string, profile model.UserInfo) (*model.User, error) {
	if providerSubject == "" {
		return nil, errors.New("provider subject cannot be empty")
	}

	user := model.User{
		ProviderSubject: providerSubject,
		Email:           profile.Email,
		EmailVerified:   profile.EmailVerified,
		Name:            profile.Name,
		GivenName:       profile.GivenName,
		FamilyName:      profile.FamilyName,
		PictureUrl:      profile.PictureUrl,
	}

	// Atomic upsert: insert or update profile fields on provider_subject conflict.
	// Eliminates the TOCTOU race from the previous read-then-write approach.
	err := s.db.WithContext(ctx).
		Where("provider_subject = ?", providerSubject).
		Assign(model.User{
			Email:         profile.Email,
			EmailVerified: profile.EmailVerified,
			Name:          profile.Name,
			GivenName:     profile.GivenName,
			FamilyName:    profile.FamilyName,
			PictureUrl:    profile.PictureUrl,
		}).
		FirstOrCreate(&user).Error
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user for subject %s: %w", providerSubject, err)
	}

	return &user, nil
}
