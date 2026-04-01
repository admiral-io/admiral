package store

import (
	"context"
	"errors"
	"fmt"

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

func (s *UserStore) UpsertByProviderSubject(ctx context.Context, providerSubject string, profile model.UserInfo) (*model.User, error) {
	if providerSubject == "" {
		return nil, errors.New("provider subject cannot be empty")
	}

	var user model.User
	result := s.db.WithContext(ctx).Where("deleted_at IS NULL").First(&user, "provider_subject = ?", providerSubject)

	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to retrieve user for subject %s: %v", providerSubject, result.Error)
		}

		user = model.User{
			ProviderSubject: providerSubject,
			Email:           profile.Email,
			EmailVerified:   profile.EmailVerified,
			Name:            profile.Name,
			GivenName:       profile.GivenName,
			FamilyName:      profile.FamilyName,
			PictureUrl:      profile.PictureUrl,
		}

		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user for subject %s: %v", providerSubject, err)
		}

		return &user, nil
	}

	user.Email = profile.Email
	user.EmailVerified = profile.EmailVerified
	user.Name = profile.Name
	user.GivenName = profile.GivenName
	user.FamilyName = profile.FamilyName
	user.PictureUrl = profile.PictureUrl

	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to update user for subject %s: %v", providerSubject, err)
	}

	return &user, nil
}
