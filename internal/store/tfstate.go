package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

var ErrLockConflict = errors.New("state is locked")

type TerraformStateStore struct {
	db *gorm.DB
}

func NewTerraformStateStore(db *gorm.DB) (*TerraformStateStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &TerraformStateStore{db: db}, nil
}

func (s *TerraformStateStore) DB() *gorm.DB {
	return s.db
}

func (s *TerraformStateStore) Create(ctx context.Context, st *model.TerraformState) (*model.TerraformState, error) {
	if err := st.Validate(); err != nil {
		return nil, fmt.Errorf("invalid terraform state: %w", err)
	}
	if err := s.db.WithContext(ctx).Create(st).Error; err != nil {
		return nil, fmt.Errorf("failed to create terraform state: %w", err)
	}
	return st, nil
}

func (s *TerraformStateStore) GetLatest(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformState, error) {
	var st model.TerraformState
	err := s.db.WithContext(ctx).
		Where("component_id = ? AND environment_id = ? AND storage_path != ''", componentID, environmentID).
		Order("created_at DESC").
		First(&st).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest terraform state: %w", err)
	}
	return &st, nil
}

func (s *TerraformStateStore) SetStoragePath(ctx context.Context, id uuid.UUID, path string) error {
	result := s.db.WithContext(ctx).
		Model(&model.TerraformState{}).
		Where("id = ?", id).
		Update("storage_path", path)
	if result.Error != nil {
		return fmt.Errorf("failed to update storage path: %w", result.Error)
	}
	return nil
}

func (s *TerraformStateStore) AcquireLock(ctx context.Context, lock *model.TerraformStateLock) (*model.TerraformStateLock, error) {
	if err := lock.Validate(); err != nil {
		return nil, fmt.Errorf("invalid terraform state lock: %w", err)
	}
	err := s.db.WithContext(ctx).Create(lock).Error
	if err != nil {
		// Check for unique/PK violation → lock already held.
		existing, loadErr := s.GetLock(ctx, lock.ComponentId, lock.EnvironmentId)
		if loadErr != nil {
			return nil, fmt.Errorf("failed to acquire lock and failed to load existing: %w", loadErr)
		}
		if existing != nil {
			return existing, ErrLockConflict
		}
		return nil, fmt.Errorf("failed to acquire terraform state lock: %w", err)
	}
	return lock, nil
}

func (s *TerraformStateStore) GetLock(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformStateLock, error) {
	var lock model.TerraformStateLock
	err := s.db.WithContext(ctx).
		Where("component_id = ? AND environment_id = ?", componentID, environmentID).
		First(&lock).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform state lock: %w", err)
	}
	return &lock, nil
}

func (s *TerraformStateStore) ReleaseLock(ctx context.Context, componentID, environmentID uuid.UUID, lockID string) error {
	result := s.db.WithContext(ctx).
		Where("component_id = ? AND environment_id = ? AND lock_id = ?", componentID, environmentID, lockID).
		Delete(&model.TerraformStateLock{})
	if result.Error != nil {
		return fmt.Errorf("failed to release terraform state lock: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// Either no lock exists or the lock_id doesn't match.
		existing, err := s.GetLock(ctx, componentID, environmentID)
		if err != nil {
			return fmt.Errorf("failed to check existing lock: %w", err)
		}
		if existing != nil {
			return ErrLockConflict
		}
		// No lock at all — treat as success (idempotent unlock).
	}
	return nil
}

// HasStateForEnv reports whether any component in the environment has
// stored terraform state (rows with non-empty storage_path). Used to gate
// destructive operations on the env -- deleting an env with state would
// orphan cloud resources.
func (s *TerraformStateStore) HasStateForEnv(ctx context.Context, envID uuid.UUID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.TerraformState{}).
		Where("environment_id = ? AND storage_path != ''", envID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check terraform state for environment: %w", err)
	}
	return count > 0, nil
}

// HasStateForApp reports whether any component anywhere in the application
// has stored terraform state.
func (s *TerraformStateStore) HasStateForApp(ctx context.Context, appID uuid.UUID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.TerraformState{}).
		Joins("JOIN components ON components.id = terraform_states.component_id").
		Where("components.application_id = ? AND terraform_states.storage_path != ''", appID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check terraform state for application: %w", err)
	}
	return count > 0, nil
}
