package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type RevisionStore struct {
	db *gorm.DB
}

func NewRevisionStore(db *gorm.DB) (*RevisionStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &RevisionStore{db: db}, nil
}

func (s *RevisionStore) DB() *gorm.DB {
	return s.db
}

func (s *RevisionStore) Create(ctx context.Context, r *model.Revision) (*model.Revision, error) {
	if err := s.db.WithContext(ctx).Create(r).Error; err != nil {
		return nil, fmt.Errorf("failed to create revision: %w", err)
	}
	return r, nil
}

func (s *RevisionStore) Get(ctx context.Context, id uuid.UUID) (*model.Revision, error) {
	var r model.Revision
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("revision not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}
	return &r, nil
}

func (s *RevisionStore) ListByDeployment(ctx context.Context, deploymentID uuid.UUID) ([]model.Revision, error) {
	var revisions []model.Revision
	err := s.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID).
		Order("created_at ASC").
		Find(&revisions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list revisions: %w", err)
	}
	return revisions, nil
}

func (s *RevisionStore) ListByDeploymentAndStatus(ctx context.Context, deploymentID uuid.UUID, status string) ([]model.Revision, error) {
	var revisions []model.Revision
	err := s.db.WithContext(ctx).
		Where("deployment_id = ? AND status = ?", deploymentID, status).
		Order("created_at ASC").
		Find(&revisions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list revisions by status: %w", err)
	}
	return revisions, nil
}

func (s *RevisionStore) Update(ctx context.Context, r *model.Revision, fields map[string]any) (*model.Revision, error) {
	result := s.db.WithContext(ctx).Model(r).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update revision: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("revision not found: %s", r.Id)
	}
	return s.Get(ctx, r.Id)
}
