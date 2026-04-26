package model

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	deploymentv1 "go.admiral.io/sdk/proto/admiral/deployment/v1"
)

const (
	DeploymentStatusPending         = "PENDING"
	DeploymentStatusQueued          = "QUEUED"
	DeploymentStatusRunning         = "RUNNING"
	DeploymentStatusSucceeded       = "SUCCEEDED"
	DeploymentStatusPartiallyFailed = "PARTIALLY_FAILED"
	DeploymentStatusFailed          = "FAILED"
	DeploymentStatusCanceled       = "CANCELED"
)

const (
	DeploymentTriggerManual  = "MANUAL"
	DeploymentTriggerCI      = "CI"
	DeploymentTriggerDestroy = "DESTROY"
)

var deploymentStatusToProto = map[string]deploymentv1.DeploymentStatus{
	DeploymentStatusPending:         deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_PENDING,
	DeploymentStatusQueued:          deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_QUEUED,
	DeploymentStatusRunning:         deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_RUNNING,
	DeploymentStatusSucceeded:       deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_SUCCEEDED,
	DeploymentStatusPartiallyFailed: deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_PARTIALLY_FAILED,
	DeploymentStatusFailed:          deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_FAILED,
	DeploymentStatusCanceled:       deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_CANCELED,
}

var deploymentTriggerToProto = map[string]deploymentv1.DeploymentTriggerType{
	DeploymentTriggerManual:  deploymentv1.DeploymentTriggerType_DEPLOYMENT_TRIGGER_TYPE_MANUAL,
	DeploymentTriggerCI:      deploymentv1.DeploymentTriggerType_DEPLOYMENT_TRIGGER_TYPE_CI,
	DeploymentTriggerDestroy: deploymentv1.DeploymentTriggerType_DEPLOYMENT_TRIGGER_TYPE_DESTROY,
}

func DeploymentStatusToProtoEnum(s string) deploymentv1.DeploymentStatus {
	if e, ok := deploymentStatusToProto[s]; ok {
		return e
	}
	return deploymentv1.DeploymentStatus_DEPLOYMENT_STATUS_UNSPECIFIED
}

type Deployment struct {
	Id                 uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ApplicationId      uuid.UUID  `gorm:"type:uuid;not null;index"`
	EnvironmentId      uuid.UUID  `gorm:"type:uuid;not null;index"`
	Status             string     `gorm:"not null"`
	TriggerType        string     `gorm:"not null"`
	TriggeredBy        string     `gorm:"not null"`
	Message            string     `gorm:"type:text;not null;default:''"`
	Destroy            bool       `gorm:"not null;default:false"`
	SourceDeploymentId  *uuid.UUID `gorm:"type:uuid"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	CompletedAt         *time.Time
	TriggeredByName     string `gorm:"->;column:triggered_by_name"`
	TriggeredByEmail    string `gorm:"->;column:triggered_by_email"`
}

func (d *Deployment) ToProto(summary *deploymentv1.RevisionSummary) *deploymentv1.Deployment {
	proto := &deploymentv1.Deployment{
		Id:              d.Id.String(),
		ApplicationId:   d.ApplicationId.String(),
		EnvironmentId:   d.EnvironmentId.String(),
		Status:          deploymentStatusToProto[d.Status],
		TriggerType:     deploymentTriggerToProto[d.TriggerType],
		TriggeredBy:     &commonv1.ActorRef{Id: d.TriggeredBy, DisplayName: d.TriggeredByName, Email: d.TriggeredByEmail},
		Message:         d.Message,
		Destroy:         d.Destroy,
		RevisionSummary: summary,
		CreatedAt:       timestamppb.New(d.CreatedAt),
	}
	if d.SourceDeploymentId != nil {
		proto.SourceDeploymentId = d.SourceDeploymentId.String()
	}
	if d.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*d.CompletedAt)
	}
	return proto
}

func DeriveRevisionSummary(revisions []Revision) *deploymentv1.RevisionSummary {
	s := &deploymentv1.RevisionSummary{Total: int32(len(revisions))}
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

func DeriveDeploymentStatus(revisions []Revision) string {
	if len(revisions) == 0 {
		return DeploymentStatusPending
	}
	var succeeded, failed, blocked, canceled int
	var inProgress bool
	for i := range revisions {
		switch revisions[i].Status {
		case RevisionStatusPending, RevisionStatusQueued,
			RevisionStatusPlanning, RevisionStatusAwaitingApproval,
			RevisionStatusApplying:
			inProgress = true
		case RevisionStatusSucceeded:
			succeeded++
		case RevisionStatusFailed:
			failed++
		case RevisionStatusBlocked:
			blocked++
		case RevisionStatusCanceled:
			canceled++
		}
	}
	if inProgress {
		return DeploymentStatusRunning
	}
	total := len(revisions)
	if succeeded == total {
		return DeploymentStatusSucceeded
	}
	if canceled == total {
		return DeploymentStatusCanceled
	}
	if failed+blocked == total {
		return DeploymentStatusFailed
	}
	return DeploymentStatusPartiallyFailed
}

func IsTerminalDeploymentStatus(s string) bool {
	switch s {
	case DeploymentStatusSucceeded,
		DeploymentStatusFailed,
		DeploymentStatusPartiallyFailed,
		DeploymentStatusCanceled:
		return true
	}
	return false
}
