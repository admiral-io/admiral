package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type ApplicationStore struct {
	db *gorm.DB
}

func NewApplicationStore(db *gorm.DB) (*ApplicationStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &ApplicationStore{db: db}, nil
}

func (s *ApplicationStore) DB() *gorm.DB {
	return s.db
}

func (s *ApplicationStore) Create(ctx context.Context, app *model.Application) (*model.Application, error) {
	if err := s.db.WithContext(ctx).Create(app).Error; err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	return app, nil
}

func (s *ApplicationStore) Get(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	var app model.Application
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&app).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("application not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	return &app, nil
}

func (s *ApplicationStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Application, error) {
	var apps []model.Application
	err := s.db.WithContext(ctx).Scopes(scopes...).Find(&apps).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	return apps, nil
}

func (s *ApplicationStore) Update(ctx context.Context, app *model.Application, fields map[string]any) (*model.Application, error) {
	result := s.db.WithContext(ctx).Model(app).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update application: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("application not found: %s", app.Id)
	}

	// Reload to get the updated record.
	return s.Get(ctx, app.Id)
}

func (s *ApplicationStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Application{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete application: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("application not found: %s", id)
	}

	return nil
}
