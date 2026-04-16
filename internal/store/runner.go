package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type RunnerStore struct {
	db *gorm.DB
}

func NewRunnerStore(db *gorm.DB) (*RunnerStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &RunnerStore{db: db}, nil
}

func (s *RunnerStore) DB() *gorm.DB {
	return s.db
}

func (s *RunnerStore) Create(ctx context.Context, r *model.Runner) (*model.Runner, error) {
	if err := s.db.WithContext(ctx).Create(r).Error; err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}
	return r, nil
}

func (s *RunnerStore) Get(ctx context.Context, id uuid.UUID) (*model.Runner, error) {
	var r model.Runner
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("runner not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get runner: %w", err)
	}
	return &r, nil
}

func (s *RunnerStore) GetByName(ctx context.Context, name string) (*model.Runner, error) {
	var r model.Runner
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("runner not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get runner by name: %w", err)
	}
	return &r, nil
}

func (s *RunnerStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Runner, error) {
	var runners []model.Runner
	err := s.db.WithContext(ctx).Scopes(scopes...).Find(&runners).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list runners: %w", err)
	}
	return runners, nil
}

func (s *RunnerStore) Update(ctx context.Context, r *model.Runner, fields map[string]any) (*model.Runner, error) {
	result := s.db.WithContext(ctx).Model(r).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update runner: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("runner not found: %s", r.Id)
	}
	return s.Get(ctx, r.Id)
}

func (s *RunnerStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Runner{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete runner: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("runner not found: %s", id)
	}
	return nil
}

func (s *RunnerStore) UpdateHeartbeat(ctx context.Context, id uuid.UUID, status *model.RunnerStatus, instanceID uuid.UUID, at time.Time) error {
	result := s.db.WithContext(ctx).
		Model(&model.Runner{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_heartbeat_at": at,
			"last_status":       status,
			"last_instance_id":  instanceID,
			"updated_at":        at,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to update heartbeat: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("runner not found: %s", id)
	}
	return nil
}
