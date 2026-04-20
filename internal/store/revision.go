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

func (s *RevisionStore) LastDeployed(ctx context.Context, componentID, environmentID uuid.UUID) (*model.Revision, error) {
	var rev model.Revision
	err := s.db.WithContext(ctx).
		Joins("JOIN deployments ON deployments.id = revisions.deployment_id").
		Where("revisions.component_id = ? AND deployments.environment_id = ? AND revisions.status = ?",
			componentID, environmentID, model.RevisionStatusSucceeded).
		Order("revisions.completed_at DESC").
		First(&rev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query last deployed revision: %w", err)
	}
	return &rev, nil
}

func (s *RevisionStore) LastDeployedByAppEnv(ctx context.Context, applicationID, environmentID uuid.UUID) (map[uuid.UUID]*model.Revision, error) {
	// Use a subquery to find the max completed_at per component, then fetch
	// the full revision rows.
	var revisions []model.Revision
	err := s.db.WithContext(ctx).
		Raw(`
			SELECT DISTINCT ON (r.component_id) r.*
			FROM revisions r
			JOIN deployments d ON d.id = r.deployment_id
			WHERE d.application_id = ?
			  AND d.environment_id = ?
			  AND r.status = ?
			ORDER BY r.component_id, r.completed_at DESC
		`, applicationID, environmentID, model.RevisionStatusSucceeded).
		Scan(&revisions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query last deployed revisions: %w", err)
	}
	result := make(map[uuid.UUID]*model.Revision, len(revisions))
	for i := range revisions {
		result[revisions[i].ComponentId] = &revisions[i]
	}
	return result, nil
}

func (s *RevisionStore) CancelNonTerminal(ctx context.Context, deploymentID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Model(&model.Revision{}).
		Where("deployment_id = ? AND status NOT IN ?",
			deploymentID,
			[]string{model.RevisionStatusSucceeded, model.RevisionStatusFailed, model.RevisionStatusCanceled},
		).
		Updates(map[string]any{
			"status":       model.RevisionStatusCanceled,
			"completed_at": time.Now(),
		}).Error
}

func (s *RevisionStore) CancelStaleAwaitingApproval(ctx context.Context, componentID, environmentID, excludeDeploymentID uuid.UUID) (int64, error) {
	result := s.db.WithContext(ctx).
		Model(&model.Revision{}).
		Where(`component_id = ? AND status = ? AND deployment_id != ?
			AND deployment_id IN (
				SELECT id FROM deployments WHERE environment_id = ?
			)`,
			componentID, model.RevisionStatusAwaitingApproval,
			excludeDeploymentID, environmentID,
		).
		Updates(map[string]any{
			"status":        model.RevisionStatusCanceled,
			"error_message": "canceled: state invalidated by a newer apply",
			"completed_at":  time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cancel stale revisions: %w", result.Error)
	}
	return result.RowsAffected, nil
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
