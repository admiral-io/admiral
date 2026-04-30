package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	environmentv1 "go.admiral.io/sdk/proto/admiral/environment/v1"
)

const (
	WorkloadTargetTypeKubernetes      = "KUBERNETES"
	InfrastructureTargetTypeTerraform = "TERRAFORM"
)

type WorkloadTarget struct {
	Type      string  `json:"type"`
	ClusterId string  `json:"cluster_id,omitempty"`
	Namespace *string `json:"namespace,omitempty"`
}

type InfrastructureTarget struct {
	Type     string `json:"type"`
	RunnerId string `json:"runner_id,omitempty"`
}

type WorkloadTargets []WorkloadTarget

func (t WorkloadTargets) Value() (driver.Value, error) {
	if t == nil {
		return "[]", nil
	}
	b, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workload targets: %w", err)
	}
	return string(b), nil
}

func (t *WorkloadTargets) Scan(value any) error {
	if value == nil {
		*t = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for WorkloadTargets: %T", value)
	}
	return json.Unmarshal(bytes, t)
}

type InfrastructureTargets []InfrastructureTarget

func (t InfrastructureTargets) Value() (driver.Value, error) {
	if t == nil {
		return "[]", nil
	}
	b, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal infrastructure targets: %w", err)
	}
	return string(b), nil
}

func (t *InfrastructureTargets) Scan(value any) error {
	if value == nil {
		*t = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for InfrastructureTargets: %T", value)
	}
	return json.Unmarshal(bytes, t)
}

func (env *Environment) Validate() error {
	if env.ApplicationId == uuid.Nil {
		return fmt.Errorf("application_id is required")
	}
	if err := ValidateSlug(env.Name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}
	if err := env.Labels.Validate(); err != nil {
		return err
	}
	if env.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}
	return nil
}

type Environment struct {
	Id                    uuid.UUID             `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ApplicationId         uuid.UUID             `gorm:"type:uuid;not null;index"`
	Name                  string                `gorm:"not null"`
	Description           string                `gorm:"type:text"`
	WorkloadTargets       WorkloadTargets       `gorm:"type:jsonb;default:'[]'"`
	InfrastructureTargets InfrastructureTargets `gorm:"type:jsonb;default:'[]'"`
	Labels                Labels                `gorm:"type:jsonb;default:'{}'"`
	CreatedBy             string                `gorm:"not null"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
	CreatedByName         string         `gorm:"->;column:created_by_name"`
	CreatedByEmail        string         `gorm:"->;column:created_by_email"`
}

func (env *Environment) ToProto(pendingChanges bool) *environmentv1.Environment {
	e := &environmentv1.Environment{
		Id:                env.Id.String(),
		ApplicationId:     env.ApplicationId.String(),
		Name:              env.Name,
		Description:       env.Description,
		Labels:            map[string]string(env.Labels),
		HasPendingChanges: pendingChanges,
		CreatedBy:         &commonv1.ActorRef{Id: env.CreatedBy, DisplayName: env.CreatedByName, Email: env.CreatedByEmail},
		CreatedAt:         timestamppb.New(env.CreatedAt),
		UpdatedAt:         timestamppb.New(env.UpdatedAt),
	}

	for _, wt := range env.WorkloadTargets {
		switch wt.Type {
		case WorkloadTargetTypeKubernetes:
			k := &environmentv1.KubernetesConfig{ClusterId: wt.ClusterId}
			if wt.Namespace != nil {
				k.Namespace = wt.Namespace
			}
			e.WorkloadTargets = append(e.WorkloadTargets, &environmentv1.WorkloadTarget{
				Config: &environmentv1.WorkloadTarget_Kubernetes{Kubernetes: k},
			})
		}
	}

	for _, it := range env.InfrastructureTargets {
		switch it.Type {
		case InfrastructureTargetTypeTerraform:
			e.InfrastructureTargets = append(e.InfrastructureTargets, &environmentv1.InfrastructureTarget{
				Config: &environmentv1.InfrastructureTarget_Terraform{
					Terraform: &environmentv1.TerraformConfig{RunnerId: it.RunnerId},
				},
			})
		}
	}

	return e
}

func (e *Environment) TerraformRunnerID() (uuid.UUID, error) {
	for _, t := range e.InfrastructureTargets {
		if t.Type != InfrastructureTargetTypeTerraform || t.RunnerId == "" {
			continue
		}
		id, err := uuid.Parse(t.RunnerId)
		if err != nil {
			return uuid.Nil, fmt.Errorf("environment %s has invalid runner_id: %v", e.Id, err)
		}
		return id, nil
	}
	return uuid.Nil, fmt.Errorf("environment %s has no terraform runner configured", e.Id)
}

func WorkloadTargetsFromProto(targets []*environmentv1.WorkloadTarget) WorkloadTargets {
	if len(targets) == 0 {
		return nil
	}
	result := make(WorkloadTargets, 0, len(targets))
	for _, t := range targets {
		if k := t.GetKubernetes(); k != nil {
			result = append(result, WorkloadTarget{
				Type:      WorkloadTargetTypeKubernetes,
				ClusterId: k.ClusterId,
				Namespace: k.Namespace,
			})
		}
	}
	return result
}

func InfrastructureTargetsFromProto(targets []*environmentv1.InfrastructureTarget) InfrastructureTargets {
	if len(targets) == 0 {
		return nil
	}
	result := make(InfrastructureTargets, 0, len(targets))
	for _, t := range targets {
		if tf := t.GetTerraform(); tf != nil {
			result = append(result, InfrastructureTarget{
				Type:     InfrastructureTargetTypeTerraform,
				RunnerId: tf.RunnerId,
			})
		}
	}
	return result
}
