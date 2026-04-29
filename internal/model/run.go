package model

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	runv1 "go.admiral.io/sdk/proto/admiral/run/v1"
)

const (
	RunStatusPending         = "PENDING"
	RunStatusQueued          = "QUEUED"
	RunStatusPlanning        = "PLANNING"
	RunStatusPlanned         = "PLANNED"
	RunStatusApplying        = "APPLYING"
	RunStatusSucceeded       = "SUCCEEDED"
	RunStatusPartiallyFailed = "PARTIALLY_FAILED"
	RunStatusFailed          = "FAILED"
	RunStatusCanceled        = "CANCELED"
	RunStatusSuperseded      = "SUPERSEDED"
)

var runStatusToProto = map[string]runv1.RunStatus{
	RunStatusPending:         runv1.RunStatus_RUN_STATUS_PENDING,
	RunStatusQueued:          runv1.RunStatus_RUN_STATUS_QUEUED,
	RunStatusPlanning:        runv1.RunStatus_RUN_STATUS_PLANNING,
	RunStatusPlanned:         runv1.RunStatus_RUN_STATUS_PLANNED,
	RunStatusApplying:        runv1.RunStatus_RUN_STATUS_APPLYING,
	RunStatusSucceeded:       runv1.RunStatus_RUN_STATUS_SUCCEEDED,
	RunStatusPartiallyFailed: runv1.RunStatus_RUN_STATUS_PARTIALLY_FAILED,
	RunStatusFailed:          runv1.RunStatus_RUN_STATUS_FAILED,
	RunStatusCanceled:        runv1.RunStatus_RUN_STATUS_CANCELED,
	RunStatusSuperseded:      runv1.RunStatus_RUN_STATUS_SUPERSEDED,
}

type Run struct {
	Id               uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ApplicationId    uuid.UUID  `gorm:"type:uuid;not null;index"`
	EnvironmentId    uuid.UUID  `gorm:"type:uuid;not null;index"`
	Status           string     `gorm:"not null"`
	TriggeredBy      string     `gorm:"not null"`
	Message          string     `gorm:"type:text;not null;default:''"`
	Destroy          bool       `gorm:"not null;default:false"`
	SourceRunId      *uuid.UUID `gorm:"type:uuid"`
	ChangeSetId      *uuid.UUID `gorm:"type:uuid"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	TriggeredByName  string `gorm:"->;column:triggered_by_name"`
	TriggeredByEmail string `gorm:"->;column:triggered_by_email"`
}

func (r *Run) ToProto(summary *runv1.RevisionSummary) *runv1.Run {
	proto := &runv1.Run{
		Id:              r.Id.String(),
		ApplicationId:   r.ApplicationId.String(),
		EnvironmentId:   r.EnvironmentId.String(),
		Status:          runStatusToProto[r.Status],
		TriggeredBy:     &commonv1.ActorRef{Id: r.TriggeredBy, DisplayName: r.TriggeredByName, Email: r.TriggeredByEmail},
		Message:         r.Message,
		Destroy:         r.Destroy,
		RevisionSummary: summary,
		CreatedAt:       timestamppb.New(r.CreatedAt),
	}
	if r.SourceRunId != nil {
		proto.SourceRunId = r.SourceRunId.String()
	}
	if r.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*r.CompletedAt)
	}
	return proto
}

func DeriveRevisionSummary(revisions []Revision) *runv1.RevisionSummary {
	s := &runv1.RevisionSummary{Total: int32(len(revisions))}
	for i := range revisions {
		switch revisions[i].Status {
		case RevisionStatusSucceeded:
			s.Succeeded++
		case RevisionStatusFailed:
			s.Failed++
		case RevisionStatusBlocked:
			s.Blocked++
		case RevisionStatusPlanning, RevisionStatusApplying:
			s.Running++
		case RevisionStatusCanceled:
			s.Canceled++
		case RevisionStatusPending, RevisionStatusQueued, RevisionStatusAwaitingApproval:
			s.Pending++
		}
	}
	return s
}

func DeriveRunStatus(revisions []Revision) string {
	if len(revisions) == 0 {
		return RunStatusPending
	}
	var succeeded, failed, blocked, canceled, superseded int
	var hasApplying, hasPlanning, hasAwaitingApproval bool
	for i := range revisions {
		switch revisions[i].Status {
		case RevisionStatusApplying:
			hasApplying = true
		case RevisionStatusPending, RevisionStatusQueued, RevisionStatusPlanning, RevisionStatusBlocked:
			hasPlanning = true
		case RevisionStatusAwaitingApproval:
			hasAwaitingApproval = true
		case RevisionStatusSucceeded:
			succeeded++
		case RevisionStatusFailed:
			failed++
		case RevisionStatusCanceled:
			canceled++
		case RevisionStatusSuperseded:
			superseded++
		}
	}
	if hasApplying {
		return RunStatusApplying
	}
	if hasPlanning {
		return RunStatusPlanning
	}
	if hasAwaitingApproval {
		return RunStatusPlanned
	}
	total := len(revisions)
	if succeeded == total {
		return RunStatusSucceeded
	}
	if superseded == total {
		return RunStatusSuperseded
	}
	if canceled == total {
		return RunStatusCanceled
	}
	if failed+blocked == total || failed+blocked+canceled == total {
		return RunStatusFailed
	}
	return RunStatusPartiallyFailed
}

func IsTerminalRunStatus(s string) bool {
	switch s {
	case RunStatusSucceeded,
		RunStatusFailed,
		RunStatusPartiallyFailed,
		RunStatusCanceled,
		RunStatusSuperseded:
		return true
	}
	return false
}

// IsActiveRunStatus reports whether a run is in a state that can be
// auto-superseded by a newer plan against the same change set or
// invalidated by an edit to the change set. APPLYING is intentionally
// excluded -- mid-apply, the system commits to finishing what's running.
func IsActiveRunStatus(s string) bool {
	switch s {
	case RunStatusPending,
		RunStatusQueued,
		RunStatusPlanning,
		RunStatusPlanned:
		return true
	}
	return false
}
