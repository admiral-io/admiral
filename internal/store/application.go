package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type ApplicationStore struct {
	db             *gorm.DB
	runStore       *RunStore
	componentStore *ComponentStore
	tfStateStore   *TerraformStateStore
}

func NewApplicationStore(db *gorm.DB) (*ApplicationStore, error) {
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

	return &ApplicationStore{
		db:             db,
		runStore:       runStore,
		componentStore: componentStore,
		tfStateStore:   tfStateStore,
	}, nil
}

func (s *ApplicationStore) DB() *gorm.DB {
	return s.db
}

func (s *ApplicationStore) Create(ctx context.Context, app *model.Application) (*model.Application, error) {
	if err := s.db.WithContext(ctx).Create(app).Error; err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	return app, nil
}

func (s *ApplicationStore) Get(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	var app model.Application
	err := s.db.WithContext(ctx).Scopes(WithActorRef("applications", "created_by")).Where("applications.id = ?", id).Take(&app).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("application not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	return &app, nil
}

func (s *ApplicationStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Application, error) {
	var apps []model.Application
	err := s.db.WithContext(ctx).Scopes(append(scopes, WithActorRef("applications", "created_by"))...).Find(&apps).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	return apps, nil
}

func (s *ApplicationStore) Update(ctx context.Context, app *model.Application, fields map[string]any) (*model.Application, error) {
	result := s.db.WithContext(ctx).Model(app).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update application: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("application not found: %s", app.Id)
	}

	return s.Get(ctx, app.Id)
}

// Delete deletes an application after checking the same blocker set as
// EnvironmentStore.Delete, scoped across every environment in the
// application:
//
//   - In-flight runs anywhere in the app: always blocks.
//   - Components with DeletionProtection=true anywhere in the app: always
//     blocks; cannot be bypassed by `force`.
//   - Components with stored terraform state anywhere in the app: soft
//     block; `force=true` bypasses (operator accepts the leak).
//
// Environments and run history cascade silently in the transaction below.
func (s *ApplicationStore) Delete(ctx context.Context, id uuid.UUID, force bool) (*DeleteResult, error) {
	app, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if hasInFlight, err := s.runStore.HasInFlightForApp(ctx, id); err != nil {
		return nil, err
	} else if hasInFlight {
		return nil, &DependentsError{
			Resource: "application",
			Name:     app.Name,
			Children: []string{"in-flight runs"},
		}
	}

	if hasProtected, err := s.componentStore.HasProtectedForApp(ctx, id); err != nil {
		return nil, err
	} else if hasProtected {
		return nil, &DependentsError{
			Resource: "application",
			Name:     app.Name,
			Children: []string{"components with deletion_protection=true"},
		}
	}

	if !force {
		if hasState, err := s.tfStateStore.HasStateForApp(ctx, id); err != nil {
			return nil, err
		} else if hasState {
			return nil, &DependentsError{
				Resource: "application",
				Name:     app.Name,
				Children: []string{"components with deployed terraform state (use force to delete; cloud resources will leak)"},
			}
		}
	}

	result := &DeleteResult{}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Runs FK is ON DELETE RESTRICT, so manual delete before envs/app
		// can go. Components/overrides/terraform_states cascade via their
		// FKs to environments.
		r := tx.Where("application_id = ?", id).Delete(&model.Run{})
		if r.Error != nil {
			return fmt.Errorf("failed to delete runs: %w", r.Error)
		}
		result.Runs = r.RowsAffected

		env := tx.Where("application_id = ?", id).Delete(&model.Environment{})
		if env.Error != nil {
			return fmt.Errorf("failed to delete environments: %w", env.Error)
		}
		result.Environments = env.RowsAffected

		appResult := tx.Where("id = ?", id).Delete(&model.Application{})
		if appResult.Error != nil {
			return fmt.Errorf("failed to delete application: %w", appResult.Error)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}
