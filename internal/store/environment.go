package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type EnvironmentStore struct {
	db             *gorm.DB
	runStore       *RunStore
	componentStore *ComponentStore
	tfStateStore   *TerraformStateStore
}

func NewEnvironmentStore(db *gorm.DB) (*EnvironmentStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	runStore, err := NewRunStore(db)
	if err != nil {
		return nil, err
	}
	componentStore, err := NewComponentStore(db)
	if err != nil {
		return nil, err
	}
	tfStateStore, err := NewTerraformStateStore(db)
	if err != nil {
		return nil, err
	}

	return &EnvironmentStore{
		db:             db,
		runStore:       runStore,
		componentStore: componentStore,
		tfStateStore:   tfStateStore,
	}, nil
}

func (s *EnvironmentStore) DB() *gorm.DB {
	return s.db
}

func (s *EnvironmentStore) Create(ctx context.Context, env *model.Environment) (*model.Environment, error) {
	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("invalid environment: %w", err)
	}
	if err := s.db.WithContext(ctx).Create(env).Error; err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	return env, nil
}

func (s *EnvironmentStore) Get(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	var env model.Environment
	err := s.db.WithContext(ctx).Scopes(WithActorRef("environments", "created_by")).Where("environments.id = ?", id).Take(&env).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("environment not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	return &env, nil
}

func (s *EnvironmentStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Environment, error) {
	var envs []model.Environment
	err := s.db.WithContext(ctx).Scopes(append(scopes, WithActorRef("environments", "created_by"))...).Find(&envs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	return envs, nil
}

func (s *EnvironmentStore) Update(ctx context.Context, env *model.Environment, fields map[string]any) (*model.Environment, error) {
	result := s.db.WithContext(ctx).Model(env).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update environment: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("environment not found: %s", env.Id)
	}

	return s.Get(ctx, env.Id)
}

// Delete deletes an environment. Blockers (returned as DependentsError):
//   - In-flight runs (PENDING/QUEUED/PLANNING/PLANNED/APPLYING): hard block.
//   - Components with DeletionProtection=true: hard block.
//   - Components with stored terraform state: soft block; `force` bypasses
//     (operator accepts the cloud-resource leak).
//
// Run history is cascaded inline; components and terraform_states cascade
// via FK. Returns DeleteResult.Runs only -- the Environments field is left
// zero so the signature matches ApplicationStore.Delete.
func (s *EnvironmentStore) Delete(ctx context.Context, id uuid.UUID, force bool) (*DeleteResult, error) {
	env, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if hasInFlight, err := s.runStore.HasInFlightForEnv(ctx, id); err != nil {
		return nil, err
	} else if hasInFlight {
		return nil, &DependentsError{
			Resource: "environment",
			Name:     env.Name,
			Children: []string{"in-flight runs"},
		}
	}

	if hasProtected, err := s.componentStore.HasProtectedForEnv(ctx, env.ApplicationId, id); err != nil {
		return nil, err
	} else if hasProtected {
		return nil, &DependentsError{
			Resource: "environment",
			Name:     env.Name,
			Children: []string{"components with deletion_protection=true"},
		}
	}

	if !force {
		if hasState, err := s.tfStateStore.HasStateForEnv(ctx, id); err != nil {
			return nil, err
		} else if hasState {
			return nil, &DependentsError{
				Resource: "environment",
				Name:     env.Name,
				Children: []string{"components with deployed terraform state (use force to delete; cloud resources will leak)"},
			}
		}
	}

	result := &DeleteResult{}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Runs FK is ON DELETE RESTRICT, so manual delete before the env
		// row can go. Components/terraform_states cascade via their FKs.
		r := tx.Where("environment_id = ?", id).Delete(&model.Run{})
		if r.Error != nil {
			return fmt.Errorf("failed to delete runs: %w", r.Error)
		}
		result.Runs = r.RowsAffected

		envResult := tx.Where("id = ?", id).Delete(&model.Environment{})
		if envResult.Error != nil {
			return fmt.Errorf("failed to delete environment: %w", envResult.Error)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// HasPendingChanges reports whether any ACTIVE infrastructure component in
// (app, env) has configuration that differs from its last successful
// revision. Non-ACTIVE components (ORPHAN/DESTROY/DESTROYED) are excluded.
func (s *EnvironmentStore) HasPendingChanges(ctx context.Context, applicationID, environmentID uuid.UUID) (bool, error) {
	var components []model.Component
	if err := s.db.WithContext(ctx).
		Where("application_id = ? AND environment_id = ?", applicationID, environmentID).
		Find(&components).Error; err != nil {
		return false, fmt.Errorf("failed to list components: %w", err)
	}
	if len(components) == 0 {
		return false, nil
	}

	var revisions []model.Revision
	if err := s.db.WithContext(ctx).
		Raw(`
			SELECT DISTINCT ON (r.component_id) r.*
			FROM revisions r
			JOIN runs ru ON ru.id = r.run_id
			WHERE ru.application_id = ?
			  AND ru.environment_id = ?
			  AND r.status = ?
			ORDER BY r.component_id, r.completed_at DESC
		`, applicationID, environmentID, model.RevisionStatusSucceeded).
		Scan(&revisions).Error; err != nil {
		return false, fmt.Errorf("failed to query last deployed revisions: %w", err)
	}
	lastDeployed := make(map[uuid.UUID]*model.Revision, len(revisions))
	for i := range revisions {
		lastDeployed[revisions[i].ComponentId] = &revisions[i]
	}

	for i := range components {
		comp := components[i]
		if comp.Kind != model.ComponentKindInfrastructure {
			continue
		}
		if comp.DesiredState != model.ComponentDesiredStateActive {
			continue
		}

		rev, ok := lastDeployed[comp.Id]
		if !ok {
			return true, nil
		}

		if comp.ModuleId != rev.ModuleId {
			return true, nil
		}
		if comp.Version != rev.Version {
			return true, nil
		}
		if comp.ValuesTemplate != rev.ResolvedValues {
			return true, nil
		}
	}

	return false, nil
}
