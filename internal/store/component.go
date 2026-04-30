package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type ComponentStore struct {
	db *gorm.DB
}

func NewComponentStore(db *gorm.DB) (*ComponentStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &ComponentStore{db: db}, nil
}

func (s *ComponentStore) DB() *gorm.DB {
	return s.db
}

func (s *ComponentStore) Create(ctx context.Context, c *model.Component) (*model.Component, error) {
	if err := s.db.WithContext(ctx).Create(c).Error; err != nil {
		return nil, fmt.Errorf("failed to create component: %w", err)
	}
	return c, nil
}

func (s *ComponentStore) Get(ctx context.Context, id uuid.UUID) (*model.Component, error) {
	var c model.Component
	err := s.db.WithContext(ctx).Scopes(WithActorRef("components", "created_by")).Where("components.id = ?", id).Take(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("component not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get component: %w", err)
	}
	return &c, nil
}

func (s *ComponentStore) GetByApplicationEnvSlug(ctx context.Context, appID, envID uuid.UUID, slug string) (*model.Component, error) {
	var c model.Component
	err := s.db.WithContext(ctx).
		Scopes(WithActorRef("components", "created_by")).
		Where("components.application_id = ? AND components.environment_id = ? AND components.slug = ?", appID, envID, slug).
		Take(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("component not found: %s/%s/%s", appID, envID, slug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get component by slug: %w", err)
	}
	return &c, nil
}

func (s *ComponentStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Component, error) {
	var cs []model.Component
	err := s.db.WithContext(ctx).Scopes(append(scopes, WithActorRef("components", "created_by"))...).Find(&cs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list components: %w", err)
	}
	return cs, nil
}

func (s *ComponentStore) ListByApplicationEnv(ctx context.Context, appID, envID uuid.UUID, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Component, error) {
	var cs []model.Component
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ?", appID, envID).
		Scopes(append(scopes, WithActorRef("components", "created_by"))...).
		Find(&cs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list components by app/env: %w", err)
	}
	return cs, nil
}

func (s *ComponentStore) Update(ctx context.Context, c *model.Component, fields map[string]any) (*model.Component, error) {
	result := s.db.WithContext(ctx).Model(c).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update component: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("component not found: %s", c.Id)
	}
	return s.Get(ctx, c.Id)
}

func (s *ComponentStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Component{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete component: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("component not found: %s", id)
	}
	return nil
}

func (s *ComponentStore) CountByModuleID(ctx context.Context, moduleID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.Component{}).
		Where("module_id = ?", moduleID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count components for module: %w", err)
	}
	return count, nil
}

func (s *ComponentStore) CountByApplicationID(ctx context.Context, appID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.Component{}).
		Where("application_id = ?", appID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count components for application: %w", err)
	}
	return count, nil
}
