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
	db *gorm.DB
}

func NewEnvironmentStore(db *gorm.DB) (*EnvironmentStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &EnvironmentStore{db: db}, nil
}

func (s *EnvironmentStore) DB() *gorm.DB {
	return s.db
}

func (s *EnvironmentStore) Create(ctx context.Context, env *model.Environment) (*model.Environment, error) {
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

func (s *EnvironmentStore) CountByApplicationID(ctx context.Context, appID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.Environment{}).
		Where("application_id = ?", appID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count environments for application: %w", err)
	}
	return count, nil
}

func (s *EnvironmentStore) NamesByApplicationID(ctx context.Context, appID uuid.UUID, limit int) ([]string, error) {
	var names []string
	err := s.db.WithContext(ctx).
		Model(&model.Environment{}).
		Where("application_id = ?", appID).
		Order("name").
		Limit(limit).
		Pluck("name", &names).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list environment names for application: %w", err)
	}
	return names, nil
}

func (s *EnvironmentStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Environment{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete environment: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("environment not found: %s", id)
	}

	return nil
}

func (s *EnvironmentStore) DeleteByApplicationID(ctx context.Context, tx *gorm.DB, appID uuid.UUID) (int64, error) {
	result := tx.WithContext(ctx).Where("application_id = ?", appID).Delete(&model.Environment{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete environments for application: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteCascade deletes an environment and its dependents. If force is false
// and deployments exist, returns a HasDependentsError.
func (s *EnvironmentStore) DeleteCascade(ctx context.Context, id uuid.UUID, force bool) (int64, error) {
	env, err := s.Get(ctx, id)
	if err != nil {
		return 0, err
	}

	var deployCount int64
	if err := s.db.WithContext(ctx).
		Model(&model.Deployment{}).
		Where("environment_id = ?", id).
		Count(&deployCount).Error; err != nil {
		return 0, fmt.Errorf("failed to count deployments: %w", err)
	}

	if deployCount > 0 && !force {
		return 0, &HasDependentsError{
			Resource: "environment",
			Name:     env.Name,
			Count:    deployCount,
			Children: []string{"deployments"},
		}
	}

	var deploymentsDeleted int64
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if deployCount > 0 {
			dep := tx.Where("environment_id = ?", id).Delete(&model.Deployment{})
			if dep.Error != nil {
				return fmt.Errorf("failed to delete deployments: %w", dep.Error)
			}
			deploymentsDeleted = dep.RowsAffected
		}

		result := tx.Where("id = ?", id).Delete(&model.Environment{})
		if result.Error != nil {
			return fmt.Errorf("failed to delete environment: %w", result.Error)
		}
		return nil
	}); err != nil {
		return 0, err
	}

	return deploymentsDeleted, nil
}

// HasPendingChanges returns true if any component in the application has
// configuration that differs from its last successfully deployed revision in
// this environment.
func (s *EnvironmentStore) HasPendingChanges(ctx context.Context, applicationID, environmentID uuid.UUID) (bool, error) {
	var components []model.Component
	if err := s.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Find(&components).Error; err != nil {
		return false, fmt.Errorf("failed to list components: %w", err)
	}
	if len(components) == 0 {
		return false, nil
	}

	var overrides []model.ComponentOverride
	if err := s.db.WithContext(ctx).
		Joins("JOIN components ON components.id = component_overrides.component_id").
		Where("components.application_id = ? AND component_overrides.environment_id = ? AND components.deleted_at IS NULL", applicationID, environmentID).
		Find(&overrides).Error; err != nil {
		return false, fmt.Errorf("failed to list overrides: %w", err)
	}
	overrideMap := make(map[uuid.UUID]*model.ComponentOverride, len(overrides))
	for i := range overrides {
		overrideMap[overrides[i].ComponentId] = &overrides[i]
	}

	var revisions []model.Revision
	if err := s.db.WithContext(ctx).
		Raw(`
			SELECT DISTINCT ON (r.component_id) r.*
			FROM revisions r
			JOIN deployments d ON d.id = r.deployment_id
			WHERE d.application_id = ?
			  AND d.environment_id = ?
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

	if len(lastDeployed) == 0 && len(components) > 0 {
		return true, nil
	}

	for i := range components {
		comp := components[i]
		if comp.Kind != model.ComponentKindInfrastructure {
			continue
		}

		if o, ok := overrideMap[comp.Id]; ok {
			if o.ApplyToModel(&comp) {
				if _, wasDeployed := lastDeployed[comp.Id]; wasDeployed {
					return true, nil
				}
				continue
			}
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
