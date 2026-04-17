package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type VariableStore struct {
	db *gorm.DB
}

func NewVariableStore(db *gorm.DB) (*VariableStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &VariableStore{db: db}, nil
}

func (s *VariableStore) DB() *gorm.DB {
	return s.db
}

func (s *VariableStore) Create(ctx context.Context, v *model.Variable) (*model.Variable, error) {
	if err := v.Validate(); err != nil {
		return nil, fmt.Errorf("invalid variable: %w", err)
	}

	if err := s.db.WithContext(ctx).Create(v).Error; err != nil {
		return nil, fmt.Errorf("failed to create variable: %w", err)
	}
	return v, nil
}

func (s *VariableStore) Get(ctx context.Context, id uuid.UUID) (*model.Variable, error) {
	var v model.Variable
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("variable not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get variable: %w", err)
	}
	return &v, nil
}

func (s *VariableStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Variable, error) {
	var vs []model.Variable
	err := s.db.WithContext(ctx).Scopes(scopes...).Find(&vs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list variables: %w", err)
	}
	return vs, nil
}

func (s *VariableStore) Update(ctx context.Context, v *model.Variable, fields map[string]any) (*model.Variable, error) {
	result := s.db.WithContext(ctx).Model(v).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update variable: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("variable not found: %s", v.Id)
	}
	return s.Get(ctx, v.Id)
}

func (s *VariableStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.Variable{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete variable: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("variable not found: %s", id)
	}
	return nil
}

func (s *VariableStore) ExistsByKey(ctx context.Context, key string, appID, envID *uuid.UUID) (bool, error) {
	q := s.db.WithContext(ctx).Model(&model.Variable{}).Where("key = ?", key)

	if envID != nil {
		q = q.Where("environment_id = ?", *envID)
	} else if appID != nil {
		q = q.Where("application_id = ? AND environment_id IS NULL", *appID)
	} else {
		q = q.Where("application_id IS NULL AND environment_id IS NULL")
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check variable key: %w", err)
	}
	return count > 0, nil
}

func (s *VariableStore) Resolve(ctx context.Context, appID, envID uuid.UUID) ([]model.Variable, error) {
	var all []model.Variable

	// Load all three layers: global, app-level, env-level.
	err := s.db.WithContext(ctx).
		Where(
			"(application_id IS NULL AND environment_id IS NULL) OR "+
				"(application_id = ? AND environment_id IS NULL) OR "+
				"(application_id = ? AND environment_id = ?)",
			appID, appID, envID,
		).
		Find(&all).Error
	if err != nil {
		return nil, fmt.Errorf("failed to resolve variables: %w", err)
	}

	// Merge: env overrides app overrides global.
	merged := make(map[string]model.Variable, len(all))
	for _, v := range all {
		existing, ok := merged[v.Key]
		if !ok {
			merged[v.Key] = v
			continue
		}
		// Higher specificity wins.
		if scopePriority(&v) > scopePriority(&existing) {
			merged[v.Key] = v
		}
	}

	result := make([]model.Variable, 0, len(merged))
	for _, v := range merged {
		result = append(result, v)
	}
	return result, nil
}

func (s *VariableStore) UpsertInfraOutputs(
	ctx context.Context,
	appID, envID uuid.UUID,
	componentName string,
	outputs []model.Variable,
) error {
	prefix := componentName + "."

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete all existing INFRASTRUCTURE variables for this component+env.
		if err := tx.
			Where("application_id = ? AND environment_id = ? AND source = ? AND key LIKE ?",
				appID, envID, model.VariableSourceInfrastructure, prefix+"%").
			Unscoped().Delete(&model.Variable{}).Error; err != nil {
			return fmt.Errorf("failed to delete stale infra outputs: %w", err)
		}

		// Insert the fresh set.
		for i := range outputs {
			if err := tx.Create(&outputs[i]).Error; err != nil {
				return fmt.Errorf("failed to create infra output %q: %w", outputs[i].Key, err)
			}
		}
		return nil
	})
}

func (s *VariableStore) DeleteInfraOutputs(
	ctx context.Context,
	appID, envID uuid.UUID,
	componentName string,
) error {
	prefix := componentName + "."
	result := s.db.WithContext(ctx).
		Unscoped().
		Where("application_id = ? AND environment_id = ? AND source = ? AND key LIKE ?",
			appID, envID, model.VariableSourceInfrastructure, prefix+"%").
		Delete(&model.Variable{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete infra outputs: %w", result.Error)
	}
	return nil
}

func scopePriority(v *model.Variable) int {
	if v.EnvironmentId != nil {
		return 2
	}
	if v.ApplicationId != nil {
		return 1
	}
	return 0
}
