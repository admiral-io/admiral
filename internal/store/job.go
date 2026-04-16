package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type JobStore struct {
	db *gorm.DB
}

func NewJobStore(db *gorm.DB) (*JobStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &JobStore{db: db}, nil
}

func (s *JobStore) DB() *gorm.DB {
	return s.db
}

func (s *JobStore) Create(ctx context.Context, j *model.Job) (*model.Job, error) {
	if err := s.db.WithContext(ctx).Create(j).Error; err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}
	return j, nil
}

func (s *JobStore) Get(ctx context.Context, id uuid.UUID) (*model.Job, error) {
	var j model.Job
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&j).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return &j, nil
}

func (s *JobStore) ListByRunner(ctx context.Context, runnerID uuid.UUID, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Job, error) {
	var jobs []model.Job
	err := s.db.WithContext(ctx).
		Where("runner_id = ?", runnerID).
		Scopes(scopes...).
		Order("created_at DESC").
		Find(&jobs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list runner jobs: %w", err)
	}
	return jobs, nil
}

func (s *JobStore) ListByRevision(ctx context.Context, revisionID uuid.UUID) ([]model.Job, error) {
	var jobs []model.Job
	err := s.db.WithContext(ctx).
		Where("revision_id = ?", revisionID).
		Order("created_at ASC").
		Find(&jobs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list revision jobs: %w", err)
	}
	return jobs, nil
}

func (s *JobStore) Update(ctx context.Context, j *model.Job, fields map[string]any) (*model.Job, error) {
	result := s.db.WithContext(ctx).Model(j).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("job not found: %s", j.Id)
	}
	return s.Get(ctx, j.Id)
}
