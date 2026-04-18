package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type DeploymentStore struct {
	db *gorm.DB
}

func NewDeploymentStore(db *gorm.DB) (*DeploymentStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &DeploymentStore{db: db}, nil
}

func (s *DeploymentStore) DB() *gorm.DB {
	return s.db
}

func (s *DeploymentStore) Create(ctx context.Context, d *model.Deployment) (*model.Deployment, error) {
	if err := s.db.WithContext(ctx).Create(d).Error; err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}
	return d, nil
}

func (s *DeploymentStore) Get(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	var d model.Deployment
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deployment not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	return &d, nil
}

func (s *DeploymentStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Deployment, error) {
	var deployments []model.Deployment
	err := s.db.WithContext(ctx).Scopes(scopes...).Order("created_at DESC").Find(&deployments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	return deployments, nil
}

func (s *DeploymentStore) FindActive(ctx context.Context, appID, envID uuid.UUID) (*model.Deployment, error) {
	var d model.Deployment
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ? AND status IN ?",
			appID, envID,
			[]string{model.DeploymentStatusPending, model.DeploymentStatusRunning},
		).
		Order("created_at ASC").
		First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find active deployment: %w", err)
	}
	return &d, nil
}

func (s *DeploymentStore) FindOldestQueued(ctx context.Context, appID, envID uuid.UUID) (*model.Deployment, error) {
	var d model.Deployment
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ? AND status = ?",
			appID, envID, model.DeploymentStatusQueued,
		).
		Order("created_at ASC").
		First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find queued deployment: %w", err)
	}
	return &d, nil
}

func (s *DeploymentStore) Update(ctx context.Context, d *model.Deployment, fields map[string]any) (*model.Deployment, error) {
	result := s.db.WithContext(ctx).Model(d).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update deployment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("deployment not found: %s", d.Id)
	}
	return s.Get(ctx, d.Id)
}
