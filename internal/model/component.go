package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
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

const (
	ComponentDesiredStateActive    = "ACTIVE"
	ComponentDesiredStateDestroy   = "DESTROY"
	ComponentDesiredStateOrphan    = "ORPHAN"
	ComponentDesiredStateDestroyed = "DESTROYED"
)

var componentKindToProto = map[string]componentv1.ComponentKind{
	ComponentKindInfrastructure: componentv1.ComponentKind_COMPONENT_KIND_INFRASTRUCTURE,
	ComponentKindWorkload:       componentv1.ComponentKind_COMPONENT_KIND_WORKLOAD,
}

var componentDesiredStateToProto = map[string]componentv1.ComponentDesiredState{
	ComponentDesiredStateActive:    componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_ACTIVE,
	ComponentDesiredStateDestroy:   componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_DESTROY,
	ComponentDesiredStateOrphan:    componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_ORPHAN,
	ComponentDesiredStateDestroyed: componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_DESTROYED,
}

var componentDesiredStateFromProto = map[componentv1.ComponentDesiredState]string{
	componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_ACTIVE:    ComponentDesiredStateActive,
	componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_DESTROY:   ComponentDesiredStateDestroy,
	componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_ORPHAN:    ComponentDesiredStateOrphan,
	componentv1.ComponentDesiredState_COMPONENT_DESIRED_STATE_DESTROYED: ComponentDesiredStateDestroyed,
}

func ComponentDesiredStateFromProto(p componentv1.ComponentDesiredState) string {
	return componentDesiredStateFromProto[p]
}

func ComponentDesiredStateToProto(s string) componentv1.ComponentDesiredState {
	return componentDesiredStateToProto[s]
}

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

type Component struct {
	Id                 uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ApplicationId      uuid.UUID        `gorm:"type:uuid;not null;index"`
	EnvironmentId      uuid.UUID        `gorm:"type:uuid;not null;index"`
	Name               string           `gorm:"not null"`
	Description        string           `gorm:"type:text"`
	Kind               string           `gorm:"not null"`
	DesiredState       string           `gorm:"not null;default:ACTIVE"`
	DeletionProtection bool             `gorm:"not null;default:false"`
	ModuleId           uuid.UUID        `gorm:"type:uuid;not null;index"`
	Version            string           `gorm:"type:text;not null;default:''"`
	ValuesTemplate     string           `gorm:"type:text;not null;default:''"`
	DependsOn          pq.StringArray   `gorm:"type:text[];not null;default:'{}'"`
	Outputs            ComponentOutputs `gorm:"type:jsonb;default:'[]'"`
	CreatedBy          string           `gorm:"not null"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          gorm.DeletedAt `gorm:"index"`
	CreatedByName      string         `gorm:"->;column:created_by_name"`
	CreatedByEmail     string         `gorm:"->;column:created_by_email"`
}

func (c *Component) Validate() error {
	if c.ApplicationId == uuid.Nil {
		return fmt.Errorf("application_id is required")
	}
	if c.EnvironmentId == uuid.Nil {
		return fmt.Errorf("environment_id is required")
	}
	if err := ValidateName(c.Name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}
	switch c.Kind {
	case ComponentKindInfrastructure, ComponentKindWorkload:
	case "":
		return fmt.Errorf("kind is required")
	default:
		return fmt.Errorf("invalid kind: %s", c.Kind)
	}
	switch c.DesiredState {
	case ComponentDesiredStateActive,
		ComponentDesiredStateDestroy,
		ComponentDesiredStateOrphan,
		ComponentDesiredStateDestroyed:
	case "":
		return fmt.Errorf("desired_state is required")
	default:
		return fmt.Errorf("invalid desired_state: %s", c.DesiredState)
	}
	if c.ModuleId == uuid.Nil {
		return fmt.Errorf("module_id is required")
	}
	if c.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}
	if err := ValidateValuesTemplate(c.ValuesTemplate); err != nil {
		return fmt.Errorf("invalid values_template: %w", err)
	}
	for _, dep := range c.DependsOn {
		if err := ValidateName(dep); err != nil {
			return fmt.Errorf("invalid depends_on entry %q: %w", dep, err)
		}
	}
	return nil
}

func (c *Component) ToProto() *componentv1.Component {
	return &componentv1.Component{
		Id:                 c.Id.String(),
		ApplicationId:      c.ApplicationId.String(),
		EnvironmentId:      c.EnvironmentId.String(),
		Name:               c.Name,
		Description:        c.Description,
		Kind:               componentKindToProto[c.Kind],
		DesiredState:       componentDesiredStateToProto[c.DesiredState],
		DeletionProtection: c.DeletionProtection,
		ModuleId:           c.ModuleId.String(),
		Version:            c.Version,
		ValuesTemplate:     c.ValuesTemplate,
		DependsOn:          []string(c.DependsOn),
		Outputs:            c.Outputs.ToProto(),
		CreatedBy:          &commonv1.ActorRef{Id: c.CreatedBy, DisplayName: c.CreatedByName, Email: c.CreatedByEmail},
		CreatedAt:          timestamppb.New(c.CreatedAt),
		UpdatedAt:          timestamppb.New(c.UpdatedAt),
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

var slugRegex = regexp.MustCompile(`^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)

func ValidateName(s string) error {
	if !slugRegex.MatchString(s) {
		return fmt.Errorf("invalid name %q: must be lowercase alphanumeric with hyphens, start with a letter", s)
	}
	return nil
}
