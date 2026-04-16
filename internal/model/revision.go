package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"google.golang.org/protobuf/types/known/timestamppb"

	deploymentv1 "go.admiral.io/sdk/proto/admiral/deployment/v1"
)

const (
	RevisionStatusPending          = "PENDING"
	RevisionStatusQueued           = "QUEUED"
	RevisionStatusPlanning         = "PLANNING"
	RevisionStatusAwaitingApproval = "AWAITING_APPROVAL"
	RevisionStatusApplying         = "APPLYING"
	RevisionStatusSucceeded        = "SUCCEEDED"
	RevisionStatusFailed           = "FAILED"
	RevisionStatusBlocked          = "BLOCKED"
	RevisionStatusCancelled        = "CANCELLED"
)

var revisionStatusToProto = map[string]deploymentv1.RevisionStatus{
	RevisionStatusPending:          deploymentv1.RevisionStatus_REVISION_STATUS_PENDING,
	RevisionStatusQueued:           deploymentv1.RevisionStatus_REVISION_STATUS_QUEUED,
	RevisionStatusPlanning:         deploymentv1.RevisionStatus_REVISION_STATUS_PLANNING,
	RevisionStatusAwaitingApproval: deploymentv1.RevisionStatus_REVISION_STATUS_AWAITING_APPROVAL,
	RevisionStatusApplying:         deploymentv1.RevisionStatus_REVISION_STATUS_APPLYING,
	RevisionStatusSucceeded:        deploymentv1.RevisionStatus_REVISION_STATUS_SUCCEEDED,
	RevisionStatusFailed:           deploymentv1.RevisionStatus_REVISION_STATUS_FAILED,
	RevisionStatusBlocked:          deploymentv1.RevisionStatus_REVISION_STATUS_BLOCKED,
	RevisionStatusCancelled:        deploymentv1.RevisionStatus_REVISION_STATUS_CANCELLED,
}

// revisionKindToProto maps the DB string form to the proto enum. Shares
// values with ComponentKind but the enum types differ.
var revisionKindToProto = map[string]deploymentv1.RevisionKind{
	ComponentKindInfrastructure: deploymentv1.RevisionKind_REVISION_KIND_INFRASTRUCTURE,
	ComponentKindWorkload:       deploymentv1.RevisionKind_REVISION_KIND_WORKLOAD,
}

type TerraformPlanSummary struct {
	Additions    int32 `json:"additions"`
	Changes      int32 `json:"changes"`
	Destructions int32 `json:"destructions"`
}

func (s TerraformPlanSummary) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan summary: %w", err)
	}
	return string(b), nil
}

func (s *TerraformPlanSummary) Scan(value any) error {
	if value == nil {
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		return fmt.Errorf("cannot scan %T into TerraformPlanSummary", value)
	}
	return json.Unmarshal(b, s)
}

func (s *TerraformPlanSummary) ToProto() *deploymentv1.TerraformPlanSummary {
	if s == nil {
		return nil
	}
	return &deploymentv1.TerraformPlanSummary{
		Additions:    s.Additions,
		Changes:      s.Changes,
		Destructions: s.Destructions,
	}
}

type Revision struct {
	Id               uuid.UUID             `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	DeploymentId     uuid.UUID             `gorm:"type:uuid;not null;index"`
	ComponentId      uuid.UUID             `gorm:"type:uuid;not null;index"`
	ComponentName    string                `gorm:"not null"`
	Kind             string                `gorm:"not null"`
	Status           string                `gorm:"not null"`
	SourceId         *uuid.UUID            `gorm:"type:uuid"`
	Version          string                `gorm:"type:text;not null;default:''"`
	ResolvedValues   string                `gorm:"type:text;not null;default:''"`
	DependsOn        pq.StringArray        `gorm:"type:text[];not null;default:'{}'"`
	BlockedBy        pq.StringArray        `gorm:"type:text[];not null;default:'{}'"`
	ArtifactChecksum string                `gorm:"type:text;not null;default:''"`
	ArtifactUrl      string                `gorm:"type:text;not null;default:''"`
	PlanOutput       string                `gorm:"type:text;not null;default:''"`
	PlanSummary      *TerraformPlanSummary `gorm:"type:jsonb"`
	ErrorMessage     string                `gorm:"type:text;not null;default:''"`
	RetryCount       int32                 `gorm:"not null;default:0"`
	CreatedAt        time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
}

func (r *Revision) ToProto() *deploymentv1.Revision {
	proto := &deploymentv1.Revision{
		Id:               r.Id.String(),
		DeploymentId:     r.DeploymentId.String(),
		ComponentId:      r.ComponentId.String(),
		ComponentName:    r.ComponentName,
		Kind:             revisionKindToProto[r.Kind],
		Status:           revisionStatusToProto[r.Status],
		Version:          r.Version,
		ResolvedValues:   r.ResolvedValues,
		DependsOn:        []string(r.DependsOn),
		BlockedBy:        []string(r.BlockedBy),
		ArtifactChecksum: r.ArtifactChecksum,
		ArtifactUrl:      r.ArtifactUrl,
		PlanOutput:       r.PlanOutput,
		PlanSummary:      r.PlanSummary.ToProto(),
		ErrorMessage:     r.ErrorMessage,
		RetryCount:       r.RetryCount,
		CreatedAt:        timestamppb.New(r.CreatedAt),
	}
	if r.SourceId != nil {
		proto.SourceId = r.SourceId.String()
	}
	if r.StartedAt != nil {
		proto.StartedAt = timestamppb.New(*r.StartedAt)
	}
	if r.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*r.CompletedAt)
	}
	return proto
}

