package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type ModuleStore struct {
	db *gorm.DB
}

func NewModuleStore(db *gorm.DB) (*ModuleStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &ModuleStore{db: db}, nil
}

func (s *ModuleStore) DB() *gorm.DB {
	return s.db
}

func (s *ModuleStore) Create(ctx context.Context, mod *model.Module) (*model.Module, error) {
	if err := mod.Validate(); err != nil {
		return nil, fmt.Errorf("invalid module: %w", err)
	}
	if err := s.db.WithContext(ctx).Create(mod).Error; err != nil {
		return nil, fmt.Errorf("failed to create module: %w", err)
	}
	return mod, nil
}

func (s *ModuleStore) Get(ctx context.Context, id uuid.UUID) (*model.Module, error) {
	var mod model.Module
	err := s.db.WithContext(ctx).Scopes(WithActorRef("modules", "created_by")).Where("modules.id = ?", id).Take(&mod).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("module not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get module: %w", err)
	}
	return &mod, nil
}

func (s *ModuleStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Module, error) {
	var mods []model.Module
	err := s.db.WithContext(ctx).Scopes(append(scopes, WithActorRef("modules", "created_by"))...).Find(&mods).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}
	return mods, nil
}

func (s *ModuleStore) Update(ctx context.Context, mod *model.Module, fields map[string]any) (*model.Module, error) {
	result := s.db.WithContext(ctx).Model(mod).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update module: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("module not found: %s", mod.Id)
	}
	return s.Get(ctx, mod.Id)
}

func (s *ModuleStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Module{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete module: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("module not found: %s", id)
	}
	return nil
}

func (s *ModuleStore) CountBySourceID(ctx context.Context, sourceID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.Module{}).
		Where("source_id = ?", sourceID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count modules for source: %w", err)
	}
	return count, nil
}
