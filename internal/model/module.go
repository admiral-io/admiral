package model

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	modulev1 "go.admiral.io/sdk/proto/admiral/module/v1"
)

const (
	ModuleTypeTerraform = "TERRAFORM"
	ModuleTypeHelm      = "HELM"
	ModuleTypeKustomize = "KUSTOMIZE"
	ModuleTypeManifest  = "MANIFEST"
)

var moduleTypeToProto = map[string]modulev1.ModuleType{
	ModuleTypeTerraform: modulev1.ModuleType_MODULE_TYPE_TERRAFORM,
	ModuleTypeHelm:      modulev1.ModuleType_MODULE_TYPE_HELM,
	ModuleTypeKustomize: modulev1.ModuleType_MODULE_TYPE_KUSTOMIZE,
	ModuleTypeManifest:  modulev1.ModuleType_MODULE_TYPE_MANIFEST,
}

var moduleTypeFromProto = map[modulev1.ModuleType]string{
	modulev1.ModuleType_MODULE_TYPE_TERRAFORM: ModuleTypeTerraform,
	modulev1.ModuleType_MODULE_TYPE_HELM:      ModuleTypeHelm,
	modulev1.ModuleType_MODULE_TYPE_KUSTOMIZE: ModuleTypeKustomize,
	modulev1.ModuleType_MODULE_TYPE_MANIFEST:  ModuleTypeManifest,
}

func ModuleTypeFromProto(t modulev1.ModuleType) string {
	return moduleTypeFromProto[t]
}

func ModuleTypeToProto(t string) (modulev1.ModuleType, bool) {
	v, ok := moduleTypeToProto[t]
	return v, ok
}

var moduleSourceCompat = map[string]map[string]bool{
	ModuleTypeTerraform: {
		SourceTypeGit:       true,
		SourceTypeTerraform: true,
		SourceTypeHTTP:      true,
	},
	ModuleTypeHelm: {
		SourceTypeGit:  true,
		SourceTypeHelm: true,
		SourceTypeOCI:  true,
		SourceTypeHTTP: true,
	},
	ModuleTypeKustomize: {
		SourceTypeGit:  true,
		SourceTypeHTTP: true,
	},
	ModuleTypeManifest: {
		SourceTypeGit:  true,
		SourceTypeHTTP: true,
	},
}

type Module struct {
	Id             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name           string    `gorm:"uniqueIndex;not null"`
	Description    string    `gorm:"type:text"`
	Type           string    `gorm:"not null"`
	SourceId       uuid.UUID `gorm:"type:uuid;not null"`
	Ref            string    `gorm:"type:text;not null;default:''"`
	Root           string    `gorm:"type:text;not null;default:''"`
	Path           string    `gorm:"type:text;not null;default:''"`
	Labels         Labels    `gorm:"type:jsonb;default:'{}'"`
	CreatedBy      string    `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
	CreatedByName  string         `gorm:"->;column:created_by_name"`
	CreatedByEmail string         `gorm:"->;column:created_by_email"`
	SourceName     string         `gorm:"->;column:source_id_name"`
}

var moduleNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*(/[a-z][a-z0-9-]*)*$`)

func (m *Module) Validate() error {
	if !moduleNameRegex.MatchString(m.Name) {
		return fmt.Errorf("invalid name %q: must be lowercase alphanumeric with hyphens, optionally separated by /", m.Name)
	}
	switch m.Type {
	case ModuleTypeTerraform, ModuleTypeHelm, ModuleTypeKustomize, ModuleTypeManifest:
	case "":
		return fmt.Errorf("type is required")
	default:
		return fmt.Errorf("invalid type: %s", m.Type)
	}
	if m.SourceId == uuid.Nil {
		return fmt.Errorf("source_id is required")
	}
	if err := m.Labels.Validate(); err != nil {
		return err
	}
	if m.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}
	return nil
}

func (m *Module) ToProto() *modulev1.Module {
	return &modulev1.Module{
		Id:          m.Id.String(),
		Name:        m.Name,
		Description: m.Description,
		Type:        moduleTypeToProto[m.Type],
		SourceId:    m.SourceId.String(),
		SourceName:  m.SourceName,
		Ref:         m.Ref,
		Root:        m.Root,
		Path:        m.Path,
		Labels:      map[string]string(m.Labels),
		CreatedBy:   &commonv1.ActorRef{Id: m.CreatedBy, DisplayName: m.CreatedByName, Email: m.CreatedByEmail},
		CreatedAt:   timestamppb.New(m.CreatedAt),
		UpdatedAt:   timestamppb.New(m.UpdatedAt),
	}
}

func ValidateModuleSourceCompat(modType, srcType string) error {
	if modType == "" {
		return fmt.Errorf("module type is required")
	}
	if srcType == "" {
		return fmt.Errorf("source type is required")
	}
	compat, ok := moduleSourceCompat[modType]
	if !ok {
		return fmt.Errorf("unsupported module type: %s", modType)
	}
	if !compat[srcType] {
		return fmt.Errorf("module type %s is not compatible with source type %s", modType, srcType)
	}
	return nil
}
