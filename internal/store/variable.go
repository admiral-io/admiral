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
	err := s.db.WithContext(ctx).Scopes(WithActorRef("variables", "created_by")).Where("variables.id = ?", id).Take(&v).Error
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
	err := s.db.WithContext(ctx).Scopes(append(scopes, WithActorRef("variables", "created_by"))...).Find(&vs).Error
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

// Resolve returns every variable in (app, env) without pagination or scopes.
// Used by orchestration to materialize the full variable set for runtime
// substitution; List is the paginated, scope-driven counterpart used by the
// env endpoint.
func (s *VariableStore) Resolve(ctx context.Context, appID, envID uuid.UUID) ([]model.Variable, error) {
	var all []model.Variable
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ?", appID, envID).
		Find(&all).Error
	if err != nil {
		return nil, fmt.Errorf("failed to resolve variables: %w", err)
	}
	return all, nil
}

func (s *VariableStore) UpsertEnvVariable(
	ctx context.Context,
	appID, envID uuid.UUID,
	key, value, varType string,
	sensitive bool,
	createdBy string,
) (*model.Variable, error) {
	var existing model.Variable
	err := s.db.WithContext(ctx).
		Where("environment_id = ? AND key = ?", envID, key).
		Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to find existing env variable: %w", err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		v := &model.Variable{
			Key:           key,
			Value:         value,
			Type:          varType,
			Source:        model.VariableSourceUser,
			Sensitive:     sensitive,
			ApplicationId: appID,
			EnvironmentId: envID,
			CreatedBy:     createdBy,
		}
		return s.Create(ctx, v)
	}
	return s.Update(ctx, &existing, map[string]any{
		"value":     value,
		"type":      varType,
		"sensitive": sensitive,
	})
}

func (s *VariableStore) DeleteByEnvKey(ctx context.Context, envID uuid.UUID, key string) error {
	result := s.db.WithContext(ctx).
		Where("environment_id = ? AND key = ?", envID, key).
		Delete(&model.Variable{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete env variable: %w", result.Error)
	}
	return nil
}

func (s *VariableStore) UpsertInfraOutputs(
	ctx context.Context,
	appID, envID uuid.UUID,
	componentSlug string,
	outputs []model.Variable,
) error {
	prefix := componentSlug + "."

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete all existing INFRASTRUCTURE variables for this component+env.
		if err := tx.
			Where("application_id = ? AND environment_id = ? AND source = ? AND key LIKE ?",
				appID, envID, model.VariableSourceInfrastructure, prefix+"%").
			Delete(&model.Variable{}).Error; err != nil {
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
	componentSlug string,
) error {
	prefix := componentSlug + "."
	result := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ? AND source = ? AND key LIKE ?",
			appID, envID, model.VariableSourceInfrastructure, prefix+"%").
		Delete(&model.Variable{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete infra outputs: %w", result.Error)
	}
	return nil
}
