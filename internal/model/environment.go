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

// WorkloadTarget is the DB-storable representation of a workload target binding.
type WorkloadTarget struct {
	Type      string  `json:"type"`
	ClusterId string  `json:"cluster_id,omitempty"`
	Namespace *string `json:"namespace,omitempty"`
}

// InfrastructureTarget is the DB-storable representation of an infrastructure target binding.
type InfrastructureTarget struct {
	Type     string `json:"type"`
	RunnerId string `json:"runner_id,omitempty"`
}

// WorkloadTargets is a JSONB-backed slice.
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

// InfrastructureTargets is a JSONB-backed slice.
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

type Environment struct {
	Id                    uuid.UUID             `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ApplicationId         uuid.UUID             `gorm:"type:uuid;not null;index"`
	Name                  string                `gorm:"not null"`
	Description           string                `gorm:"type:text"`
	WorkloadTargets       WorkloadTargets       `gorm:"type:jsonb;default:'[]'"`
	InfrastructureTargets InfrastructureTargets `gorm:"type:jsonb;default:'[]'"`
	Labels                Labels                `gorm:"type:jsonb;default:'{}'"`
	HasPendingChanges     bool                  `gorm:"not null;default:false"`
	LastDeployedAt        *time.Time            `gorm:"type:timestamptz"`
	CreatedBy             string                `gorm:"not null"`
	UpdatedBy             string                `gorm:"not null"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

func (env *Environment) ToProto() *environmentv1.Environment {
	e := &environmentv1.Environment{
		Id:                env.Id.String(),
		ApplicationId:     env.ApplicationId.String(),
		Name:              env.Name,
		Description:       env.Description,
		Labels:            map[string]string(env.Labels),
		HasPendingChanges: env.HasPendingChanges,
		CreatedBy:         &commonv1.ActorRef{Id: env.CreatedBy},
		UpdatedBy:         &commonv1.ActorRef{Id: env.UpdatedBy},
		CreatedAt:         timestamppb.New(env.CreatedAt),
		UpdatedAt:         timestamppb.New(env.UpdatedAt),
	}

	for _, wt := range env.WorkloadTargets {
		target := &environmentv1.WorkloadTarget{}
		switch wt.Type {
		case "kubernetes":
			k := &environmentv1.KubernetesConfig{ClusterId: wt.ClusterId}
			if wt.Namespace != nil {
				k.Namespace = wt.Namespace
			}
			target.Config = &environmentv1.WorkloadTarget_Kubernetes{Kubernetes: k}
		}
		e.WorkloadTargets = append(e.WorkloadTargets, target)
	}

	for _, it := range env.InfrastructureTargets {
		target := &environmentv1.InfrastructureTarget{}
		switch it.Type {
		case "terraform":
			target.Config = &environmentv1.InfrastructureTarget_Terraform{
				Terraform: &environmentv1.TerraformConfig{RunnerId: it.RunnerId},
			}
		}
		e.InfrastructureTargets = append(e.InfrastructureTargets, target)
	}

	if env.LastDeployedAt != nil {
		e.LastDeployedAt = timestamppb.New(*env.LastDeployedAt)
	}

	return e
}

// WorkloadTargetsFromProto converts proto workload targets to the DB model.
func WorkloadTargetsFromProto(targets []*environmentv1.WorkloadTarget) WorkloadTargets {
	if len(targets) == 0 {
		return nil
	}
	result := make(WorkloadTargets, 0, len(targets))
	for _, t := range targets {
		if k := t.GetKubernetes(); k != nil {
			result = append(result, WorkloadTarget{
				Type:      "kubernetes",
				ClusterId: k.ClusterId,
				Namespace: k.Namespace,
			})
		}
	}
	return result
}

// InfrastructureTargetsFromProto converts proto infrastructure targets to the DB model.
func InfrastructureTargetsFromProto(targets []*environmentv1.InfrastructureTarget) InfrastructureTargets {
	if len(targets) == 0 {
		return nil
	}
	result := make(InfrastructureTargets, 0, len(targets))
	for _, t := range targets {
		if tf := t.GetTerraform(); tf != nil {
			result = append(result, InfrastructureTarget{
				Type:     "terraform",
				RunnerId: tf.RunnerId,
			})
		}
	}
	return result
}
