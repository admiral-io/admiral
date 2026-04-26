package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

type HasDependentsError struct {
	Resource string
	Name     string
	Count    int64
	Children []string
}

func (e *HasDependentsError) Error() string {
	return fmt.Sprintf("cannot delete %s %q: %d dependent(s) still exist (%s); delete them first or use force",
		e.Resource, e.Name, e.Count, strings.Join(e.Children, ", "))
}

type DeleteResult struct {
	Environments int64
	Deployments  int64
}

type ApplicationStore struct {
	db *gorm.DB
}

func NewApplicationStore(db *gorm.DB) (*ApplicationStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &ApplicationStore{db: db}, nil
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

func (s *ApplicationStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Application{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete application: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("application not found: %s", id)
	}

	return nil
}

func (s *ApplicationStore) DeleteCascade(ctx context.Context, id uuid.UUID, force bool) (*DeleteResult, error) {
	app, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	var envCount int64
	if err := s.db.WithContext(ctx).
		Model(&model.Environment{}).
		Where("application_id = ?", id).
		Count(&envCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count environments: %w", err)
	}

	if envCount > 0 && !force {
		var names []string
		_ = s.db.WithContext(ctx).
			Model(&model.Environment{}).
			Where("application_id = ?", id).
			Order("name").Limit(10).
			Pluck("name", &names).Error
		return nil, &HasDependentsError{
			Resource: "application",
			Name:     app.Name,
			Count:    envCount,
			Children: names,
		}
	}

	result := &DeleteResult{}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if envCount > 0 {
			dep := tx.Where("application_id = ?", id).Delete(&model.Deployment{})
			if dep.Error != nil {
				return fmt.Errorf("failed to delete deployments: %w", dep.Error)
			}
			result.Deployments = dep.RowsAffected

			env := tx.Where("application_id = ?", id).Delete(&model.Environment{})
			if env.Error != nil {
				return fmt.Errorf("failed to delete environments: %w", env.Error)
			}
			result.Environments = env.RowsAffected
		}

		app := tx.Where("id = ?", id).Delete(&model.Application{})
		if app.Error != nil {
			return fmt.Errorf("failed to delete application: %w", app.Error)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}
