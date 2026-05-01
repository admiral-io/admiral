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
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("invalid revision: %w", err)
	}
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

func (s *RevisionStore) ListByRun(ctx context.Context, runID uuid.UUID) ([]model.Revision, error) {
	var revisions []model.Revision
	err := s.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&revisions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list revisions: %w", err)
	}
	return revisions, nil
}

func (s *RevisionStore) ListByRunAndStatus(ctx context.Context, runID uuid.UUID, status string) ([]model.Revision, error) {
	var revisions []model.Revision
	err := s.db.WithContext(ctx).
		Where("run_id = ? AND status = ?", runID, status).
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
		Joins("JOIN runs ON runs.id = revisions.run_id").
		Where("revisions.component_id = ? AND runs.environment_id = ? AND revisions.status = ?",
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

// LatestSucceededByEnv returns a slug-keyed map of the most recent SUCCEEDED
// revision id per component for an (application, environment). Used by
// change set conflict detection: snapshot at change set creation, compare
// per-entry at deploy time. Slugs not present in the map mean the component
// has never been successfully deployed in this env.
func (s *RevisionStore) LatestSucceededByEnv(ctx context.Context, applicationID, environmentID uuid.UUID) (map[string]uuid.UUID, error) {
	type row struct {
		Slug       string    `gorm:"column:slug"`
		RevisionId uuid.UUID `gorm:"column:revision_id"`
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Raw(`
			SELECT DISTINCT ON (r.component_id) c.slug AS slug, r.id AS revision_id
			FROM revisions r
			JOIN runs ru ON ru.id = r.run_id
			JOIN components c ON c.id = r.component_id
			WHERE ru.application_id = ?
			  AND ru.environment_id = ?
			  AND r.status = ?
			ORDER BY r.component_id, r.completed_at DESC
		`, applicationID, environmentID, model.RevisionStatusSucceeded).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query latest succeeded revisions by env: %w", err)
	}
	result := make(map[string]uuid.UUID, len(rows))
	for _, r := range rows {
		result[r.Slug] = r.RevisionId
	}
	return result, nil
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

func (s *RevisionStore) CancelNonTerminal(ctx context.Context, runID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Model(&model.Revision{}).
		Where("run_id = ? AND status NOT IN ?",
			runID,
			[]string{model.RevisionStatusSucceeded, model.RevisionStatusFailed, model.RevisionStatusCanceled},
		).
		Updates(map[string]any{
			"status":       model.RevisionStatusCanceled,
			"completed_at": time.Now(),
		}).Error
}

// SupersedeNonTerminal transitions all non-terminal revisions for a run to
// SUPERSEDED. Used when the parent run is replaced by a newer plan for the
// same change set, or invalidated by a change-set edit. Distinct from
// cancel: cancel is user-initiated, supersede is system-driven.
func (s *RevisionStore) SupersedeNonTerminal(ctx context.Context, runID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Model(&model.Revision{}).
		Where("run_id = ? AND status NOT IN ?",
			runID,
			[]string{
				model.RevisionStatusSucceeded,
				model.RevisionStatusFailed,
				model.RevisionStatusCanceled,
				model.RevisionStatusSuperseded,
			}).
		Updates(map[string]any{
			"status":       model.RevisionStatusSuperseded,
			"completed_at": time.Now(),
		}).Error
}

func (s *RevisionStore) CancelStaleAwaitingApproval(ctx context.Context, componentID, environmentID, excludeRunID uuid.UUID) (int64, error) {
	result := s.db.WithContext(ctx).
		Model(&model.Revision{}).
		Where(`component_id = ? AND status = ? AND run_id != ?
			AND run_id IN (
				SELECT id FROM runs WHERE environment_id = ?
			)`,
			componentID, model.RevisionStatusAwaitingApproval,
			excludeRunID, environmentID,
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
