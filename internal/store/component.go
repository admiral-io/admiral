package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("component not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get component: %w", err)
	}
	return &c, nil
}

func (s *ComponentStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Component, error) {
	var cs []model.Component
	err := s.db.WithContext(ctx).Scopes(scopes...).Find(&cs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list components: %w", err)
	}
	return cs, nil
}

func (s *ComponentStore) ListByApplicationID(ctx context.Context, appID uuid.UUID, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Component, error) {
	var cs []model.Component
	err := s.db.WithContext(ctx).
		Where("application_id = ?", appID).
		Scopes(scopes...).
		Find(&cs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list components by application: %w", err)
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

type ComponentOverrideStore struct {
	db *gorm.DB
}

func NewComponentOverrideStore(db *gorm.DB) (*ComponentOverrideStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &ComponentOverrideStore{db: db}, nil
}

// Set upserts an override by (component_id, environment_id).
func (s *ComponentOverrideStore) Set(ctx context.Context, o *model.ComponentOverride) (*model.ComponentOverride, error) {
	err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "component_id"}, {Name: "environment_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"disabled", "module_id", "version", "values_template",
				"depends_on", "outputs", "updated_at",
			}),
		}).
		Create(o).Error
	if err != nil {
		return nil, fmt.Errorf("failed to set component override: %w", err)
	}
	return s.Get(ctx, o.ComponentId, o.EnvironmentId)
}

func (s *ComponentOverrideStore) Get(ctx context.Context, componentID, environmentID uuid.UUID) (*model.ComponentOverride, error) {
	var o model.ComponentOverride
	err := s.db.WithContext(ctx).
		Where("component_id = ? AND environment_id = ?", componentID, environmentID).
		First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("component override not found: %s/%s", componentID, environmentID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get component override: %w", err)
	}
	return &o, nil
}

func (s *ComponentOverrideStore) ListByComponentID(ctx context.Context, componentID uuid.UUID) ([]model.ComponentOverride, error) {
	var os []model.ComponentOverride
	err := s.db.WithContext(ctx).
		Where("component_id = ?", componentID).
		Find(&os).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list component overrides: %w", err)
	}
	return os, nil
}

func (s *ComponentOverrideStore) ListByApplicationEnv(ctx context.Context, appID, envID uuid.UUID) ([]model.ComponentOverride, error) {
	var os []model.ComponentOverride
	err := s.db.WithContext(ctx).
		Joins("JOIN components ON components.id = component_overrides.component_id").
		Where("components.application_id = ? AND component_overrides.environment_id = ? AND components.deleted_at IS NULL", appID, envID).
		Find(&os).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list component overrides for app/env: %w", err)
	}
	return os, nil
}

func (s *ComponentOverrideStore) Delete(ctx context.Context, componentID, environmentID uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Where("component_id = ? AND environment_id = ?", componentID, environmentID).
		Delete(&model.ComponentOverride{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete component override: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("component override not found: %s/%s", componentID, environmentID)
	}
	return nil
}
