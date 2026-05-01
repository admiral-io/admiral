package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type RunStore struct {
	db *gorm.DB
}

func NewRunStore(db *gorm.DB) (*RunStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &RunStore{db: db}, nil
}

func (s *RunStore) DB() *gorm.DB {
	return s.db
}

func (s *RunStore) Create(ctx context.Context, r *model.Run) (*model.Run, error) {
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("invalid run: %w", err)
	}
	if err := s.db.WithContext(ctx).Create(r).Error; err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}
	return r, nil
}

func (s *RunStore) Get(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	var r model.Run
	err := s.db.WithContext(ctx).Scopes(WithActorRef("runs", "triggered_by")).Where("runs.id = ?", id).Take(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	return &r, nil
}

func (s *RunStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Run, error) {
	var runs []model.Run
	err := s.db.WithContext(ctx).Scopes(append(scopes, WithActorRef("runs", "triggered_by"))...).Order("created_at DESC").Find(&runs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	return runs, nil
}

func (s *RunStore) Update(ctx context.Context, r *model.Run, fields map[string]any) (*model.Run, error) {
	result := s.db.WithContext(ctx).Model(r).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update run: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("run not found: %s", r.Id)
	}
	return s.Get(ctx, r.Id)
}

// FindActive returns the in-progress run for (app, env) if one exists.
// QUEUED is intentionally excluded: this answers "is something running --
// should I queue behind it?" A QUEUED peer would just have the new run join
// the queue, so it isn't blocking. Use HasInFlightForEnv when the question
// is "is anything unfinished" (e.g. delete blockers), which does include
// QUEUED.
func (s *RunStore) FindActive(ctx context.Context, appID, envID uuid.UUID) (*model.Run, error) {
	var r model.Run
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ? AND status IN ?",
			appID, envID,
			[]string{model.RunStatusPending, model.RunStatusPlanning, model.RunStatusPlanned, model.RunStatusApplying},
		).
		Order("created_at ASC").
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find active run: %w", err)
	}
	return &r, nil
}

// FindActiveByChangeSet returns the change set's currently-active run
// (PENDING/QUEUED/PLANNING/PLANNED/APPLYING) if one exists. APPLYING is
// included here because callers need to detect mid-flight applies and reject
// supersede/edit attempts; the per-status policy is enforced by callers.
func (s *RunStore) FindActiveByChangeSet(ctx context.Context, changeSetID uuid.UUID) (*model.Run, error) {
	var r model.Run
	err := s.db.WithContext(ctx).
		Where("change_set_id = ? AND status IN ?",
			changeSetID,
			[]string{
				model.RunStatusPending,
				model.RunStatusQueued,
				model.RunStatusPlanning,
				model.RunStatusPlanned,
				model.RunStatusApplying,
			}).
		Order("created_at DESC").
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find active run for change set: %w", err)
	}
	return &r, nil
}

func (s *RunStore) FindOldestQueued(ctx context.Context, appID, envID uuid.UUID) (*model.Run, error) {
	var r model.Run
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ? AND status = ?",
			appID, envID, model.RunStatusQueued,
		).
		Order("created_at ASC").
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find queued run: %w", err)
	}
	return &r, nil
}

// inFlightStatuses lists run statuses that block destructive operations on
// the surrounding env/app -- the engine may still be holding state.
var inFlightStatuses = []string{
	model.RunStatusPending,
	model.RunStatusQueued,
	model.RunStatusPlanning,
	model.RunStatusPlanned,
	model.RunStatusApplying,
}

// HasInFlightForEnv reports whether the given environment has any run
// currently mid-flight (PENDING/QUEUED/PLANNING/PLANNED/APPLYING).
func (s *RunStore) HasInFlightForEnv(ctx context.Context, envID uuid.UUID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.Run{}).
		Where("environment_id = ? AND status IN ?", envID, inFlightStatuses).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check in-flight runs for environment: %w", err)
	}
	return count > 0, nil
}

// HasInFlightForApp reports whether any run in the application (across all
// environments) is currently mid-flight.
func (s *RunStore) HasInFlightForApp(ctx context.Context, appID uuid.UUID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.Run{}).
		Where("application_id = ? AND status IN ?", appID, inFlightStatuses).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check in-flight runs for application: %w", err)
	}
	return count > 0, nil
}
