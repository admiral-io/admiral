package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"google.golang.org/protobuf/types/known/timestamppb"

	changesetv1 "go.admiral.io/sdk/proto/admiral/changeset/v1"
	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
)

type ChangeSetBaseHead map[string]uuid.UUID

func (b ChangeSetBaseHead) Value() (driver.Value, error) {
	if b == nil {
		return "{}", nil
	}
	bs, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal base_head_revisions: %w", err)
	}
	return string(bs), nil
}

func (b *ChangeSetBaseHead) Scan(value any) error {
	if value == nil {
		*b = ChangeSetBaseHead{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for ChangeSetBaseHead: %T", value)
	}
	return json.Unmarshal(bytes, b)
}

const (
	ChangeSetStatusOpen      = "OPEN"
	ChangeSetStatusDeployed  = "DEPLOYED"
	ChangeSetStatusDiscarded = "DISCARDED"
)

const (
	ChangeSetEntryTypeCreate  = "CREATE"
	ChangeSetEntryTypeUpdate  = "UPDATE"
	ChangeSetEntryTypeDestroy = "DESTROY"
	ChangeSetEntryTypeOrphan  = "ORPHAN"
)

// DisplayIDPrefixChangeSet is the typed prefix for changeset display IDs
// (e.g. cs-3k7m9p2q4rvw). Used with displayid.Generate / displayid.Is.
const DisplayIDPrefixChangeSet = "cs"

func IsTerminalChangeSetStatus(s string) bool {
	return s == ChangeSetStatusDeployed || s == ChangeSetStatusDiscarded
}

func ValidateChangeSetEntryType(t string) error {
	switch t {
	case ChangeSetEntryTypeCreate, ChangeSetEntryTypeUpdate,
		ChangeSetEntryTypeDestroy, ChangeSetEntryTypeOrphan:
		return nil
	case "":
		return fmt.Errorf("change_type is required")
	default:
		return fmt.Errorf("invalid change_type: %s", t)
	}
}

type ChangeSet struct {
	Id                uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	DisplayId         string            `gorm:"type:text;not null;uniqueIndex;column:display_id"`
	ApplicationId     uuid.UUID         `gorm:"type:uuid;not null;index"`
	EnvironmentId     uuid.UUID         `gorm:"type:uuid;not null;index"`
	Status            string            `gorm:"not null;default:OPEN"`
	CopiedFromId      *uuid.UUID        `gorm:"type:uuid"`
	Title             string            `gorm:"type:text;not null;default:''"`
	Description       string            `gorm:"type:text;not null;default:''"`
	RunId             *uuid.UUID        `gorm:"type:uuid"`
	BaseHeadRevisions ChangeSetBaseHead `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedBy         string            `gorm:"not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CreatedByName     string `gorm:"->;column:created_by_name"`
	CreatedByEmail    string `gorm:"->;column:created_by_email"`
	ApplicationName   string `gorm:"->;column:application_id_name"`
	EnvironmentName   string `gorm:"->;column:environment_id_name"`
}

func (cs *ChangeSet) Validate() error {
	if cs.ApplicationId == uuid.Nil {
		return fmt.Errorf("application_id is required")
	}
	if cs.EnvironmentId == uuid.Nil {
		return fmt.Errorf("environment_id is required")
	}
	switch cs.Status {
	case ChangeSetStatusOpen, ChangeSetStatusDeployed, ChangeSetStatusDiscarded:
	case "":
		return fmt.Errorf("status is required")
	default:
		return fmt.Errorf("invalid status: %s", cs.Status)
	}
	return nil
}

func (cs *ChangeSet) RequireMutable() error {
	if cs.Status != ChangeSetStatusOpen {
		return fmt.Errorf("change set is %s and cannot be modified", cs.Status)
	}
	return nil
}

func (cs *ChangeSet) ToProto(entries []ChangeSetEntry, varEntries []ChangeSetVariableEntry) *changesetv1.ChangeSet {
	out := &changesetv1.ChangeSet{
		Id:              cs.Id.String(),
		DisplayId:       cs.DisplayId,
		ApplicationId:   cs.ApplicationId.String(),
		ApplicationName: cs.ApplicationName,
		EnvironmentId:   cs.EnvironmentId.String(),
		EnvironmentName: cs.EnvironmentName,
		Status:          cs.Status,
		Title:           cs.Title,
		Description:     cs.Description,
		CreatedBy: &commonv1.ActorRef{
			Id:          cs.CreatedBy,
			DisplayName: cs.CreatedByName,
			Email:       cs.CreatedByEmail,
		},
		CreatedAt: timestamppb.New(cs.CreatedAt),
		UpdatedAt: timestamppb.New(cs.UpdatedAt),
	}
	if cs.CopiedFromId != nil {
		out.CopiedFromId = cs.CopiedFromId.String()
	}
	if cs.RunId != nil {
		out.RunId = cs.RunId.String()
	}
	if entries != nil {
		out.Entries = make([]*changesetv1.ChangeSetEntry, 0, len(entries))
		for i := range entries {
			out.Entries = append(out.Entries, entries[i].ToProto())
		}
	}
	if varEntries != nil {
		out.VariableEntries = make([]*changesetv1.ChangeSetVariableEntry, 0, len(varEntries))
		for i := range varEntries {
			out.VariableEntries = append(out.VariableEntries, varEntries[i].ToProto())
		}
	}
	return out
}

type ChangeSetEntry struct {
	Id             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ChangeSetId    uuid.UUID      `gorm:"type:uuid;not null;index"`
	ComponentId    *uuid.UUID     `gorm:"type:uuid"`
	ComponentSlug  string         `gorm:"not null"`
	ChangeType     string         `gorm:"not null"`
	ModuleId       *uuid.UUID     `gorm:"type:uuid"`
	Version        *string        `gorm:"type:text"`
	ValuesTemplate *string        `gorm:"type:text"`
	DependsOn      pq.StringArray `gorm:"type:text[]"`
	Description    *string        `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (e *ChangeSetEntry) Validate() error {
	if e.ChangeSetId == uuid.Nil {
		return fmt.Errorf("change_set_id is required")
	}
	if err := ValidateSlug(e.ComponentSlug); err != nil {
		return fmt.Errorf("invalid component_slug: %w", err)
	}

	switch e.ChangeType {
	case ChangeSetEntryTypeCreate:
		if e.ComponentId != nil {
			return fmt.Errorf("CREATE entries must not reference an existing component")
		}
		if e.ModuleId == nil {
			return fmt.Errorf("CREATE entries require module_id")
		}
		if e.ValuesTemplate != nil {
			if err := ValidateValuesTemplate(*e.ValuesTemplate); err != nil {
				return fmt.Errorf("invalid values_template: %w", err)
			}
		}

	case ChangeSetEntryTypeUpdate:
		if e.ComponentId == nil {
			return fmt.Errorf("UPDATE entries require component_id")
		}
		if e.ModuleId == nil && e.Version == nil && e.ValuesTemplate == nil &&
			e.DependsOn == nil && e.Description == nil {
			return fmt.Errorf("UPDATE entries must change at least one field")
		}
		if e.ValuesTemplate != nil {
			if err := ValidateValuesTemplate(*e.ValuesTemplate); err != nil {
				return fmt.Errorf("invalid values_template: %w", err)
			}
		}

	case ChangeSetEntryTypeDestroy, ChangeSetEntryTypeOrphan:
		if e.ComponentId == nil {
			return fmt.Errorf("%s entries require component_id", e.ChangeType)
		}
		if e.ModuleId != nil || e.Version != nil || e.ValuesTemplate != nil ||
			e.DependsOn != nil {
			return fmt.Errorf("%s entries must not set module_id, version, values_template, or depends_on", e.ChangeType)
		}

	case "":
		return fmt.Errorf("change_type is required")
	default:
		return fmt.Errorf("invalid change_type: %s", e.ChangeType)
	}
	for _, dep := range e.DependsOn {
		if err := ValidateSlug(dep); err != nil {
			return fmt.Errorf("invalid depends_on entry %q: %w", dep, err)
		}
	}
	return nil
}

func (e *ChangeSetEntry) ToProto() *changesetv1.ChangeSetEntry {
	out := &changesetv1.ChangeSetEntry{
		Id:            e.Id.String(),
		ChangeSetId:   e.ChangeSetId.String(),
		ComponentSlug: e.ComponentSlug,
		ChangeType:    e.ChangeType,
		DependsOn:     []string(e.DependsOn),
		CreatedAt:     timestamppb.New(e.CreatedAt),
		UpdatedAt:     timestamppb.New(e.UpdatedAt),
	}
	if e.ComponentId != nil {
		out.ComponentId = e.ComponentId.String()
	}
	if e.ModuleId != nil {
		s := e.ModuleId.String()
		out.ModuleId = &s
	}
	if e.Version != nil {
		v := *e.Version
		out.Version = &v
	}
	if e.ValuesTemplate != nil {
		v := *e.ValuesTemplate
		out.ValuesTemplate = &v
	}
	if e.Description != nil {
		v := *e.Description
		out.Description = &v
	}
	return out
}

type ChangeSetVariableEntry struct {
	Id          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ChangeSetId uuid.UUID `gorm:"type:uuid;not null;index"`
	Key         string    `gorm:"type:text;not null"`
	Value       *string   `gorm:"type:text"`
	Type        string    `gorm:"type:text;not null;default:'STRING'"`
	Sensitive   bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time
}

func (v *ChangeSetVariableEntry) IsDelete() bool {
	return v.Value == nil
}

func (v *ChangeSetVariableEntry) Validate() error {
	if v.ChangeSetId == uuid.Nil {
		return fmt.Errorf("change_set_id is required")
	}
	if strings.TrimSpace(v.Key) == "" {
		return fmt.Errorf("key is required")
	}
	switch v.Type {
	case VariableTypeString, VariableTypeNumber, VariableTypeBoolean, VariableTypeComplex:
	case "":
		return fmt.Errorf("type is required")
	default:
		return fmt.Errorf("invalid type: %s", v.Type)
	}
	if v.Value != nil {
		if err := ValidateVariableValue(v.Type, *v.Value); err != nil {
			return err
		}
	}
	return nil
}

func (v *ChangeSetVariableEntry) ToProto() *changesetv1.ChangeSetVariableEntry {
	out := &changesetv1.ChangeSetVariableEntry{
		Id:          v.Id.String(),
		ChangeSetId: v.ChangeSetId.String(),
		Key:         v.Key,
		Type:        variableTypeToProto[v.Type],
		Sensitive:   v.Sensitive,
		CreatedAt:   timestamppb.New(v.CreatedAt),
	}
	if v.Value != nil && !v.Sensitive {
		val := *v.Value
		out.Value = &val
	}
	return out
}
