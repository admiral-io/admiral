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

	runv1 "go.admiral.io/sdk/proto/admiral/run/v1"
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
	RevisionStatusSuperseded       = "SUPERSEDED"
)

const (
	RevisionChangeTypeCreate   = "CREATE"
	RevisionChangeTypeUpdate   = "UPDATE"
	RevisionChangeTypeDestroy  = "DESTROY"
	RevisionChangeTypeRecreate = "RECREATE"
	RevisionChangeTypeImport   = "IMPORT"
	RevisionChangeTypeNoChange = "NO_CHANGE"
)

var revisionStatusToProto = map[string]runv1.RevisionStatus{
	RevisionStatusPending:          runv1.RevisionStatus_REVISION_STATUS_PENDING,
	RevisionStatusQueued:           runv1.RevisionStatus_REVISION_STATUS_QUEUED,
	RevisionStatusPlanning:         runv1.RevisionStatus_REVISION_STATUS_PLANNING,
	RevisionStatusAwaitingApproval: runv1.RevisionStatus_REVISION_STATUS_AWAITING_APPROVAL,
	RevisionStatusApplying:         runv1.RevisionStatus_REVISION_STATUS_APPLYING,
	RevisionStatusSucceeded:        runv1.RevisionStatus_REVISION_STATUS_SUCCEEDED,
	RevisionStatusFailed:           runv1.RevisionStatus_REVISION_STATUS_FAILED,
	RevisionStatusBlocked:          runv1.RevisionStatus_REVISION_STATUS_BLOCKED,
	RevisionStatusCanceled:         runv1.RevisionStatus_REVISION_STATUS_CANCELED,
	RevisionStatusSuperseded:       runv1.RevisionStatus_REVISION_STATUS_SUPERSEDED,
}

var revisionKindToProto = map[string]runv1.RevisionKind{
	ComponentKindInfrastructure: runv1.RevisionKind_REVISION_KIND_INFRASTRUCTURE,
	ComponentKindWorkload:       runv1.RevisionKind_REVISION_KIND_WORKLOAD,
}

type ChangeSummary struct {
	Additions    int32 `json:"additions"`
	Changes      int32 `json:"changes"`
	Destructions int32 `json:"destructions"`
}

func (s ChangeSummary) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan summary: %w", err)
	}
	return string(b), nil
}

func (s *ChangeSummary) Scan(value any) error {
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
		return fmt.Errorf("cannot scan %T into ChangeSummary", value)
	}
	return json.Unmarshal(b, s)
}

func (s *ChangeSummary) ToProto() *runv1.ChangeSummary {
	if s == nil {
		return nil
	}
	return &runv1.ChangeSummary{
		Additions:    s.Additions,
		Changes:      s.Changes,
		Destructions: s.Destructions,
	}
}

type Revision struct {
	Id                 uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	RunId              uuid.UUID      `gorm:"type:uuid;not null;index"`
	ComponentId        uuid.UUID      `gorm:"type:uuid;not null;index"`
	ComponentSlug      string         `gorm:"column:component_slug;not null"`
	Kind               string         `gorm:"not null"`
	Status             string         `gorm:"not null"`
	ChangeType         string         `gorm:"not null;default:CREATE"`
	PreviousRevisionId *uuid.UUID     `gorm:"type:uuid"`
	ModuleId           uuid.UUID      `gorm:"type:uuid;not null"`
	SourceId           *uuid.UUID     `gorm:"type:uuid"`
	Version            string         `gorm:"type:text;not null;default:''"`
	ResolvedValues     string         `gorm:"type:text;not null;default:''"`
	DependsOn          pq.StringArray `gorm:"type:text[];not null;default:'{}'"`
	BlockedBy          pq.StringArray `gorm:"type:text[];not null;default:'{}'"`
	WorkingDirectory   string         `gorm:"type:text;not null;default:''"`
	ArtifactChecksum   string         `gorm:"type:text;not null;default:''"`
	ArtifactUrl        string         `gorm:"type:text;not null;default:''"`
	PlanOutputKey      string         `gorm:"type:text;not null;default:''"`
	PlanFileKey        string         `gorm:"type:text;not null;default:''"`
	PlanSummary        *ChangeSummary `gorm:"type:jsonb"`
	ErrorMessage       string         `gorm:"type:text;not null;default:''"`
	RetryCount         int32          `gorm:"not null;default:0"`
	CreatedAt          time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
}

func (r *Revision) Validate() error {
	if r.RunId == uuid.Nil {
		return fmt.Errorf("run_id is required")
	}
	if r.ComponentId == uuid.Nil {
		return fmt.Errorf("component_id is required")
	}
	if r.ComponentSlug == "" {
		return fmt.Errorf("component_slug is required")
	}
	switch r.Kind {
	case ComponentKindInfrastructure, ComponentKindWorkload:
	case "":
		return fmt.Errorf("kind is required")
	default:
		return fmt.Errorf("invalid kind: %s", r.Kind)
	}
	switch r.Status {
	case RevisionStatusPending, RevisionStatusQueued, RevisionStatusPlanning,
		RevisionStatusAwaitingApproval, RevisionStatusApplying,
		RevisionStatusSucceeded, RevisionStatusFailed, RevisionStatusBlocked,
		RevisionStatusCanceled, RevisionStatusSuperseded:
	case "":
		return fmt.Errorf("status is required")
	default:
		return fmt.Errorf("invalid status: %s", r.Status)
	}
	switch r.ChangeType {
	case RevisionChangeTypeCreate, RevisionChangeTypeUpdate, RevisionChangeTypeDestroy,
		RevisionChangeTypeRecreate, RevisionChangeTypeImport, RevisionChangeTypeNoChange:
	case "":
		return fmt.Errorf("change_type is required")
	default:
		return fmt.Errorf("invalid change_type: %s", r.ChangeType)
	}
	if r.ModuleId == uuid.Nil {
		return fmt.Errorf("module_id is required")
	}
	return nil
}

func (r *Revision) ToProto() *runv1.Revision {
	proto := &runv1.Revision{
		Id:               r.Id.String(),
		RunId:            r.RunId.String(),
		ComponentId:      r.ComponentId.String(),
		ComponentSlug:    r.ComponentSlug,
		Kind:             revisionKindToProto[r.Kind],
		Status:           revisionStatusToProto[r.Status],
		ChangeType:       r.ChangeType,
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
	if r.PreviousRevisionId != nil {
		proto.PreviousRevisionId = r.PreviousRevisionId.String()
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
			fields["plan_summary"] = &ChangeSummary{
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
