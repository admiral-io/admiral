package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go.admiral.io/admiral/internal/displayid"
	"go.admiral.io/admiral/internal/model"
)

type ChangeSetStore struct {
	db *gorm.DB
}

func NewChangeSetStore(db *gorm.DB) (*ChangeSetStore, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	return &ChangeSetStore{db: db}, nil
}

func (s *ChangeSetStore) DB() *gorm.DB {
	return s.db
}

// displayIDInsertAttempts bounds the regenerate-on-collision loop in Create.
// With 60 bits of entropy in the suffix, three tries is several orders of
// magnitude beyond any realistic collision rate.
const displayIDInsertAttempts = 3

func (s *ChangeSetStore) Create(ctx context.Context, cs *model.ChangeSet) (*model.ChangeSet, error) {
	if err := cs.Validate(); err != nil {
		return nil, fmt.Errorf("invalid change set: %w", err)
	}
	if err := createWithDisplayID(s.db.WithContext(ctx), cs); err != nil {
		return nil, err
	}
	return s.Get(ctx, cs.Id)
}

// createWithDisplayID inserts cs with a freshly generated display ID,
// retrying up to displayIDInsertAttempts times on collision. The display
// ID is regenerated each attempt; cs.Id is set by GORM on success.
func createWithDisplayID(tx *gorm.DB, cs *model.ChangeSet) error {
	var lastErr error
	for range displayIDInsertAttempts {
		cs.DisplayId = displayid.Generate(model.DisplayIDPrefixChangeSet)
		if err := tx.Create(cs).Error; err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("failed to create change set: %w", lastErr)
}

func (s *ChangeSetStore) Get(ctx context.Context, id uuid.UUID) (*model.ChangeSet, error) {
	var cs model.ChangeSet
	err := s.db.WithContext(ctx).
		Scopes(WithActorRef("change_sets", "created_by")).
		Where("change_sets.id = ?", id).
		Take(&cs).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("change set not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get change set: %w", err)
	}
	return &cs, nil
}

// GetByIdentifier resolves either a UUID or a `cs-<suffix>` display ID to a
// ChangeSet. Used by lookup endpoints to accept both forms transparently.
func (s *ChangeSetStore) GetByIdentifier(ctx context.Context, ident string) (*model.ChangeSet, error) {
	if displayid.Is(ident, model.DisplayIDPrefixChangeSet) {
		var cs model.ChangeSet
		err := s.db.WithContext(ctx).
			Scopes(WithActorRef("change_sets", "created_by")).
			Where("change_sets.display_id = ?", ident).
			Take(&cs).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("change set not found: %s", ident)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get change set: %w", err)
		}
		return &cs, nil
	}
	id, err := uuid.Parse(ident)
	if err != nil {
		return nil, fmt.Errorf("invalid change_set_id: %s", ident)
	}
	return s.Get(ctx, id)
}

func (s *ChangeSetStore) List(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]model.ChangeSet, error) {
	var out []model.ChangeSet
	err := s.db.WithContext(ctx).
		Scopes(append(scopes, WithActorRef("change_sets", "created_by"))...).
		Order("change_sets.created_at DESC").
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list change sets: %w", err)
	}
	return out, nil
}

func (s *ChangeSetStore) Update(ctx context.Context, cs *model.ChangeSet, fields map[string]any) (*model.ChangeSet, error) {
	result := s.db.WithContext(ctx).Model(cs).Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update change set: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("change set not found: %s", cs.Id)
	}
	return s.Get(ctx, cs.Id)
}

// Copy creates a new OPEN change set in the target environment by cloning
// the source change set's entries and variable entries inside a single
// transaction. The new change set's CopiedFromId is set to the source ID.
// Title/description default to the source's values when empty.
//
// baseHeadRevisions is the snapshot of the target env's deployed state, used
// for conflict detection -- the caller (endpoint) computes it from the
// revision store at copy time. The source's base is intentionally not
// reused: the target env may have a completely different deployed state.
func (s *ChangeSetStore) Copy(ctx context.Context, source *model.ChangeSet, targetEnvID uuid.UUID, title, description, createdBy string, baseHeadRevisions model.ChangeSetBaseHead) (*model.ChangeSet, error) {
	if title == "" {
		title = source.Title
	}
	if description == "" {
		description = source.Description
	}
	if baseHeadRevisions == nil {
		baseHeadRevisions = model.ChangeSetBaseHead{}
	}
	srcID := source.Id
	dest := &model.ChangeSet{
		ApplicationId:     source.ApplicationId,
		EnvironmentId:     targetEnvID,
		Status:            model.ChangeSetStatusOpen,
		CopiedFromId:      &srcID,
		Title:             title,
		Description:       description,
		BaseHeadRevisions: baseHeadRevisions,
		CreatedBy:         createdBy,
	}
	if err := dest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid change set: %w", err)
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createWithDisplayID(tx, dest); err != nil {
			return err
		}

		var entries []model.ChangeSetEntry
		if err := tx.Where("change_set_id = ?", srcID).Order("created_at ASC").Find(&entries).Error; err != nil {
			return fmt.Errorf("failed to load source entries: %w", err)
		}
		for i := range entries {
			e := &entries[i]
			e.Id = uuid.Nil
			e.ChangeSetId = dest.Id
			// Re-resolve ComponentId for the target environment. Components
			// are env-scoped: source's ComponentId points to a row in the
			// source env and is meaningless in the target env. CREATE
			// entries materialize fresh in the target env at deploy time
			// (clear the ID); UPDATE/DESTROY/ORPHAN must point at the
			// target env's component with the same slug.
			switch e.ChangeType {
			case model.ChangeSetEntryTypeCreate:
				e.ComponentId = nil
			case model.ChangeSetEntryTypeUpdate,
				model.ChangeSetEntryTypeDestroy,
				model.ChangeSetEntryTypeOrphan:
				var targetComp model.Component
				if err := tx.Where(
					"application_id = ? AND environment_id = ? AND slug = ? AND deleted_at IS NULL",
					source.ApplicationId, targetEnvID, e.ComponentSlug,
				).Take(&targetComp).Error; err != nil {
					return fmt.Errorf("component %q does not exist in target environment: %w", e.ComponentSlug, err)
				}
				e.ComponentId = &targetComp.Id
			}
			if err := tx.Create(e).Error; err != nil {
				return fmt.Errorf("failed to copy entry: %w", err)
			}
		}

		var vars []model.ChangeSetVariableEntry
		if err := tx.Where("change_set_id = ?", srcID).Order("key ASC").Find(&vars).Error; err != nil {
			return fmt.Errorf("failed to load source variable entries: %w", err)
		}
		for i := range vars {
			vars[i].Id = uuid.Nil
			vars[i].ChangeSetId = dest.Id
			if err := tx.Create(&vars[i]).Error; err != nil {
				return fmt.Errorf("failed to copy variable entry: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, dest.Id)
}

func (s *ChangeSetStore) UpsertEntry(ctx context.Context, e *model.ChangeSetEntry) (*model.ChangeSetEntry, error) {
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("invalid entry: %w", err)
	}
	err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "change_set_id"}, {Name: "component_slug"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"component_id", "change_type", "module_id", "version",
				"values_template", "depends_on", "description", "updated_at",
			}),
		}).
		Create(e).Error
	if err != nil {
		return nil, fmt.Errorf("failed to upsert entry: %w", err)
	}
	return s.GetEntryBySlug(ctx, e.ChangeSetId, e.ComponentSlug)
}

func (s *ChangeSetStore) GetEntryBySlug(ctx context.Context, changeSetID uuid.UUID, slug string) (*model.ChangeSetEntry, error) {
	var e model.ChangeSetEntry
	err := s.db.WithContext(ctx).
		Where("change_set_id = ? AND component_slug = ?", changeSetID, slug).
		Take(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("entry not found: %s/%s: %w", changeSetID, slug, err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}
	return &e, nil
}

func (s *ChangeSetStore) ListEntries(ctx context.Context, changeSetID uuid.UUID) ([]model.ChangeSetEntry, error) {
	var out []model.ChangeSetEntry
	err := s.db.WithContext(ctx).
		Where("change_set_id = ?", changeSetID).
		Order("created_at ASC").
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}
	return out, nil
}

// SetEntryComponentID stamps a materialized component_id onto a CREATE entry
// after the deploy has created the underlying components row. Bypasses model
// validation (which forbids component_id on CREATE) intentionally: the field
// is system-managed once the entry has been materialized, so a subsequent
// deploy retry can recognize the entry as already-created and refresh HEAD
// instead of erroring with AlreadyExists.
func (s *ChangeSetStore) SetEntryComponentID(ctx context.Context, entryID, componentID uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Model(&model.ChangeSetEntry{}).
		Where("id = ?", entryID).
		Updates(map[string]any{"component_id": componentID})
	if result.Error != nil {
		return fmt.Errorf("failed to backfill component_id on entry: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("entry not found: %s", entryID)
	}
	return nil
}

func (s *ChangeSetStore) DeleteEntryBySlug(ctx context.Context, changeSetID uuid.UUID, slug string) error {
	result := s.db.WithContext(ctx).
		Where("change_set_id = ? AND component_slug = ?", changeSetID, slug).
		Delete(&model.ChangeSetEntry{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete entry: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("entry not found: %s/%s", changeSetID, slug)
	}
	return nil
}

func (s *ChangeSetStore) UpsertVariableEntry(ctx context.Context, v *model.ChangeSetVariableEntry) (*model.ChangeSetVariableEntry, error) {
	if err := v.Validate(); err != nil {
		return nil, fmt.Errorf("invalid variable entry: %w", err)
	}
	err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "change_set_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"value", "type", "sensitive",
			}),
		}).
		Create(v).Error
	if err != nil {
		return nil, fmt.Errorf("failed to upsert variable entry: %w", err)
	}
	return s.GetVariableEntryByKey(ctx, v.ChangeSetId, v.Key)
}

func (s *ChangeSetStore) GetVariableEntryByKey(ctx context.Context, changeSetID uuid.UUID, key string) (*model.ChangeSetVariableEntry, error) {
	var v model.ChangeSetVariableEntry
	err := s.db.WithContext(ctx).
		Where("change_set_id = ? AND key = ?", changeSetID, key).
		Take(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("variable entry not found: %s/%s", changeSetID, key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get variable entry: %w", err)
	}
	return &v, nil
}

func (s *ChangeSetStore) ListVariableEntries(ctx context.Context, changeSetID uuid.UUID) ([]model.ChangeSetVariableEntry, error) {
	var out []model.ChangeSetVariableEntry
	err := s.db.WithContext(ctx).
		Where("change_set_id = ?", changeSetID).
		Order("key ASC").
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list variable entries: %w", err)
	}
	return out, nil
}

