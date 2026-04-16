package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type EnvironmentStore struct {
	db *gorm.DB
}

func NewEnvironmentStore(db *gorm.DB) (*EnvironmentStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &EnvironmentStore{db: db}, nil
}

func (s *EnvironmentStore) DB() *gorm.DB {
	return s.db
}

func (s *EnvironmentStore) Create(ctx context.Context, env *model.Environment) (*model.Environment, error) {
	if err := s.db.WithContext(ctx).Create(env).Error; err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	return env, nil
}

func (s *EnvironmentStore) Get(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	var env model.Environment
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&env).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("environment not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	return &env, nil
}

func (s *EnvironmentStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Environment, error) {
	var envs []model.Environment
	err := s.db.WithContext(ctx).Scopes(scopes...).Find(&envs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	return envs, nil
}

func (s *EnvironmentStore) Update(ctx context.Context, env *model.Environment, fields map[string]any) (*model.Environment, error) {
	result := s.db.WithContext(ctx).Model(env).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update environment: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("environment not found: %s", env.Id)
	}

	return s.Get(ctx, env.Id)
}

func (s *EnvironmentStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Environment{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete environment: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("environment not found: %s", id)
	}

	return nil
}
