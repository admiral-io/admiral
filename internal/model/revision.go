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

	deploymentv1 "go.admiral.io/sdk/proto/admiral/deployment/v1"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
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
	RevisionStatusCanceled         = "CANCELED"
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
	RevisionStatusCanceled:         deploymentv1.RevisionStatus_REVISION_STATUS_CANCELED,
}

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
	ModuleId         uuid.UUID             `gorm:"type:uuid;not null"`
	SourceId         *uuid.UUID            `gorm:"type:uuid"`
	Version          string                `gorm:"type:text;not null;default:''"`
	ResolvedValues   string                `gorm:"type:text;not null;default:''"`
	DependsOn        pq.StringArray        `gorm:"type:text[];not null;default:'{}'"`
	BlockedBy        pq.StringArray        `gorm:"type:text[];not null;default:'{}'"`
	WorkingDirectory string                `gorm:"type:text;not null;default:''"`
	ArtifactChecksum string                `gorm:"type:text;not null;default:''"`
	ArtifactUrl      string                `gorm:"type:text;not null;default:''"`
	PlanOutputKey    string                `gorm:"type:text;not null;default:''"`
	PlanFileKey      string                `gorm:"type:text;not null;default:''"`
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
		ModuleId:         r.ModuleId.String(),
		Version:          r.Version,
		ResolvedValues:   r.ResolvedValues,
		DependsOn:        []string(r.DependsOn),
		BlockedBy:        []string(r.BlockedBy),
		WorkingDirectory: r.WorkingDirectory,
		ArtifactChecksum: r.ArtifactChecksum,
		ArtifactUrl:      r.ArtifactUrl,
		HasPlanOutput:    r.PlanOutputKey != "",
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

func ParseResolvedValuesAsVars(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("resolved_values is not a JSON object: %w", err)
	}
	out := make(map[string]string, len(parsed))
	for k, v := range parsed {
		switch t := v.(type) {
		case string:
			out[k] = t
		case nil:
			out[k] = ""
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("marshal var %q: %w", k, err)
			}
			out[k] = string(b)
		}
	}
	return out, nil
}

func DeriveRevisionUpdate(jobType, reportedStatus string, result *runnerv1.JobResult) map[string]any {
	fields := map[string]any{}
	if reportedStatus == JobStatusFailed {
		fields["status"] = RevisionStatusFailed
		fields["error_message"] = result.GetErrorMessage()
		return fields
	}

	switch jobType {
	case JobTypePlan, JobTypeDestroyPlan:
		fields["status"] = RevisionStatusAwaitingApproval
		fields["error_message"] = ""
		if ps := result.GetPlanSummary(); ps != nil {
			fields["plan_summary"] = &TerraformPlanSummary{
				Additions:    ps.GetAdditions(),
				Changes:      ps.GetChanges(),
				Destructions: ps.GetDestructions(),
			}
		}
	case JobTypeApply, JobTypeDestroyApply:
		fields["status"] = RevisionStatusSucceeded
		fields["error_message"] = ""
	}
	return fields
}

// IsRevisionSatisfiedFor returns true if a blocker revision's status is
// sufficient to unblock the given job type. Plan jobs are unblocked when
// blockers reach AWAITING_APPROVAL (plan complete). Apply jobs are unblocked
// when blockers reach SUCCEEDED (apply complete).
func IsRevisionSatisfiedFor(jobType, blockerStatus string) bool {
	switch jobType {
	case JobTypePlan, JobTypeDestroyPlan:
		return blockerStatus == RevisionStatusAwaitingApproval ||
			blockerStatus == RevisionStatusApplying ||
			blockerStatus == RevisionStatusSucceeded
	case JobTypeApply, JobTypeDestroyApply:
		return blockerStatus == RevisionStatusSucceeded
	}
	return false
}
