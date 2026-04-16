package model

import (
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

type Module struct {
	Id        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name      string    `gorm:"uniqueIndex;not null"`
	Description string  `gorm:"type:text"`
	Type      string    `gorm:"not null"`
	SourceId  uuid.UUID `gorm:"type:uuid;not null"`
	Ref       string    `gorm:"type:text;not null;default:''"`
	Root      string    `gorm:"type:text;not null;default:''"`
	Path      string    `gorm:"type:text;not null;default:''"`
	Labels    Labels    `gorm:"type:jsonb;default:'{}'"`
	CreatedBy string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (m *Module) ToProto() *modulev1.Module {
	return &modulev1.Module{
		Id:          m.Id.String(),
		Name:        m.Name,
		Description: m.Description,
		Type:        moduleTypeToProto[m.Type],
		SourceId:    m.SourceId.String(),
		Ref:         m.Ref,
		Root:        m.Root,
		Path:        m.Path,
		Labels:      map[string]string(m.Labels),
		CreatedBy:   &commonv1.ActorRef{Id: m.CreatedBy},
		CreatedAt:   timestamppb.New(m.CreatedAt),
		UpdatedAt:   timestamppb.New(m.UpdatedAt),
	}
}
