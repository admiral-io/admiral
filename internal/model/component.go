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
	"gorm.io/gorm"

	admtemplate "go.admiral.io/admiral/internal/template"
	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	componentv1 "go.admiral.io/sdk/proto/admiral/component/v1"
)

const (
	ComponentKindInfrastructure = "INFRASTRUCTURE"
	ComponentKindWorkload       = "WORKLOAD"
)

var componentKindToProto = map[string]componentv1.ComponentKind{
	ComponentKindInfrastructure: componentv1.ComponentKind_COMPONENT_KIND_INFRASTRUCTURE,
	ComponentKindWorkload:       componentv1.ComponentKind_COMPONENT_KIND_WORKLOAD,
}

// --- Supporting types ---

type ComponentOutput struct {
	Name          string `json:"name"`
	ValueTemplate string `json:"value_template"`
	Description   string `json:"description,omitempty"`
}

type ComponentOutputs []ComponentOutput

func (o ComponentOutputs) Value() (driver.Value, error) {
	if o == nil {
		return "[]", nil
	}
	b, err := json.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal component outputs: %w", err)
	}
	return string(b), nil
}

func (o *ComponentOutputs) Scan(value any) error {
	if value == nil {
		*o = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for ComponentOutputs: %T", value)
	}
	return json.Unmarshal(bytes, o)
}

func (o ComponentOutputs) ToProto() []*componentv1.ComponentOutput {
	if len(o) == 0 {
		return nil
	}
	result := make([]*componentv1.ComponentOutput, 0, len(o))
	for _, out := range o {
		result = append(result, &componentv1.ComponentOutput{
			Name:          out.Name,
			ValueTemplate: out.ValueTemplate,
			Description:   out.Description,
		})
	}
	return result
}

func ComponentOutputsFromProto(protos []*componentv1.ComponentOutput) ComponentOutputs {
	if len(protos) == 0 {
		return nil
	}
	result := make(ComponentOutputs, 0, len(protos))
	for _, p := range protos {
		result = append(result, ComponentOutput{
			Name:          p.GetName(),
			ValueTemplate: p.GetValueTemplate(),
			Description:   p.GetDescription(),
		})
	}
	return result
}

// --- Component ---

type Component struct {
	Id             uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ApplicationId  uuid.UUID        `gorm:"type:uuid;not null;index"`
	Name           string           `gorm:"not null"`
	Description    string           `gorm:"type:text"`
	Kind           string           `gorm:"not null"`
	ModuleId       uuid.UUID        `gorm:"type:uuid;not null;index"`
	Version        string           `gorm:"type:text;not null;default:''"`
	ValuesTemplate string           `gorm:"type:text;not null;default:''"`
	DependsOn      pq.StringArray   `gorm:"type:text[];not null;default:'{}'"`
	Outputs        ComponentOutputs `gorm:"type:jsonb;default:'[]'"`
	CreatedBy      string           `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (c *Component) ToProto() *componentv1.Component {
	return &componentv1.Component{
		Id:             c.Id.String(),
		ApplicationId:  c.ApplicationId.String(),
		Name:           c.Name,
		Description:    c.Description,
		Kind:           componentKindToProto[c.Kind],
		ModuleId:       c.ModuleId.String(),
		Version:        c.Version,
		ValuesTemplate: c.ValuesTemplate,
		DependsOn:      []string(c.DependsOn),
		Outputs:        c.Outputs.ToProto(),
		CreatedBy:      &commonv1.ActorRef{Id: c.CreatedBy},
		CreatedAt:      timestamppb.New(c.CreatedAt),
		UpdatedAt:      timestamppb.New(c.UpdatedAt),
	}
}

func DeriveComponentKind(moduleType string) string {
	switch moduleType {
	case ModuleTypeTerraform:
		return ComponentKindInfrastructure
	case ModuleTypeHelm, ModuleTypeKustomize, ModuleTypeManifest:
		return ComponentKindWorkload
	default:
		return ""
	}
}

func ValidateValuesTemplate(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return admtemplate.Validate(s)
}

func ParseDependsOn(deps []string) ([]string, error) {
	if len(deps) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(deps))
	for _, d := range deps {
		id, err := uuid.Parse(d)
		if err != nil {
			return nil, fmt.Errorf("not a valid UUID: %s", d)
		}
		out = append(out, id.String())
	}
	return out, nil
}

// --- ComponentOverride ---

type ComponentOverride struct {
	ComponentId    uuid.UUID         `gorm:"type:uuid;primaryKey"`
	EnvironmentId  uuid.UUID         `gorm:"type:uuid;primaryKey"`
	Disabled       bool              `gorm:"not null;default:false"`
	ModuleId       *uuid.UUID        `gorm:"type:uuid"`
	Version        *string           `gorm:"type:text"`
	ValuesTemplate *string           `gorm:"type:text"`
	DependsOn      pq.StringArray    `gorm:"type:text[]"`
	Outputs        *ComponentOutputs `gorm:"type:jsonb"`
	CreatedBy      string            `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (o *ComponentOverride) TableName() string {
	return "component_overrides"
}

func (o *ComponentOverride) ApplyTo(p *componentv1.Component) {
	p.Disabled = o.Disabled
	if o.Disabled {
		return
	}
	if o.ModuleId != nil {
		p.ModuleId = o.ModuleId.String()
	}
	if o.Version != nil {
		p.Version = *o.Version
	}
	if o.ValuesTemplate != nil {
		p.ValuesTemplate = *o.ValuesTemplate
	}
	if o.DependsOn != nil {
		p.DependsOn = []string(o.DependsOn)
	}
	if o.Outputs != nil {
		p.Outputs = o.Outputs.ToProto()
	}
}

func (o *ComponentOverride) ApplyToModel(c *Component) bool {
	if o.Disabled {
		return true
	}
	if o.ModuleId != nil {
		c.ModuleId = *o.ModuleId
	}
	if o.Version != nil {
		c.Version = *o.Version
	}
	if o.ValuesTemplate != nil {
		c.ValuesTemplate = *o.ValuesTemplate
	}
	if o.DependsOn != nil {
		c.DependsOn = pq.StringArray(o.DependsOn)
	}
	if o.Outputs != nil {
		c.Outputs = ComponentOutputs(*o.Outputs)
	}
	return false
}

func (o *ComponentOverride) ToProto() *componentv1.ComponentOverride {
	out := &componentv1.ComponentOverride{
		ComponentId:   o.ComponentId.String(),
		EnvironmentId: o.EnvironmentId.String(),
		Disabled:      o.Disabled,
		CreatedBy:     &commonv1.ActorRef{Id: o.CreatedBy},
		CreatedAt:     timestamppb.New(o.CreatedAt),
		UpdatedAt:     timestamppb.New(o.UpdatedAt),
	}
	if o.ModuleId != nil {
		s := o.ModuleId.String()
		out.ModuleId = &s
	}
	if o.Version != nil {
		v := *o.Version
		out.Version = &v
	}
	if o.ValuesTemplate != nil {
		v := *o.ValuesTemplate
		out.ValuesTemplate = &v
	}
	if o.DependsOn != nil {
		out.DependsOn = []string(o.DependsOn)
	}
	if o.Outputs != nil {
		out.Outputs = o.Outputs.ToProto()
	}
	return out
}
