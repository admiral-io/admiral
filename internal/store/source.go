package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

var ErrInvalidSource = errors.New("invalid source")

type SourceStore struct {
	db *gorm.DB
}

func NewSourceStore(db *gorm.DB) (*SourceStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	return &SourceStore{db: db}, nil
}

func (s *SourceStore) DB() *gorm.DB {
	return s.db
}

func (s *SourceStore) Create(ctx context.Context, src *model.Source) (*model.Source, error) {
	if err := src.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSource, err)
	}

	if err := s.db.WithContext(ctx).Create(src).Error; err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	return src, nil
}

func (s *SourceStore) Get(ctx context.Context, id uuid.UUID) (*model.Source, error) {
	var src model.Source
	err := s.db.WithContext(ctx).Scopes(sourceEnrichment()).Where("sources.id = ?", id).Take(&src).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("source not found: %s", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return &src, nil
}

func (s *SourceStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Source, error) {
	var srcs []model.Source
	err := s.db.WithContext(ctx).Scopes(append(scopes, sourceEnrichment())...).Find(&srcs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	return srcs, nil
}

// sourceEnrichment composes the LEFT JOINs that surface denormalized fields
// on every Source read: the creator's name/email and the attached
// credential's name (empty for public sources where credential_id is null).
func sourceEnrichment() func(*gorm.DB) *gorm.DB {
	return WithEnrichment("sources",
		ActorJoin("created_by"),
		NameJoin("credential_id", "credentials"),
	)
}

func (s *SourceStore) Update(ctx context.Context, src *model.Source, fields map[string]any) (*model.Source, error) {
	if sc, ok := fields["source_config"]; ok {
		cfg, ok := sc.(model.SourceConfig)
		if !ok {
			return nil, fmt.Errorf("%w: source_config must be model.SourceConfig, got %T", ErrInvalidSource, sc)
		}
		if err := cfg.Validate(src.Type); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSource, err)
		}
	}

	result := s.db.WithContext(ctx).Model(src).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update source: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("source not found: %s", src.Id)
	}

	return s.Get(ctx, src.Id)
}

func (s *SourceStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Source{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete source: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("source not found: %s", id)
	}

	return nil
}

func (s *SourceStore) CountByCredentialID(ctx context.Context, credID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.Source{}).
		Where("credential_id = ?", credID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count sources for credential: %w", err)
	}
	return count, nil
}
