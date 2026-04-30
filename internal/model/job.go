package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

const (
	JobTypePlan         = "PLAN"
	JobTypeApply        = "APPLY"
	JobTypeDestroyPlan  = "DESTROY_PLAN"
	JobTypeDestroyApply = "DESTROY_APPLY"
)

const (
	JobStatusPending   = "PENDING"
	JobStatusAssigned  = "ASSIGNED"
	JobStatusRunning   = "RUNNING"
	JobStatusSucceeded = "SUCCEEDED"
	JobStatusFailed    = "FAILED"
	JobStatusCanceled  = "CANCELED"
)

var jobTypeToProto = map[string]runnerv1.JobType{
	JobTypePlan:         runnerv1.JobType_JOB_TYPE_PLAN,
	JobTypeApply:        runnerv1.JobType_JOB_TYPE_APPLY,
	JobTypeDestroyPlan:  runnerv1.JobType_JOB_TYPE_DESTROY_PLAN,
	JobTypeDestroyApply: runnerv1.JobType_JOB_TYPE_DESTROY_APPLY,
}

var jobStatusToProto = map[string]runnerv1.JobStatus{
	JobStatusPending:   runnerv1.JobStatus_JOB_STATUS_PENDING,
	JobStatusAssigned:  runnerv1.JobStatus_JOB_STATUS_ASSIGNED,
	JobStatusRunning:   runnerv1.JobStatus_JOB_STATUS_RUNNING,
	JobStatusSucceeded: runnerv1.JobStatus_JOB_STATUS_SUCCEEDED,
	JobStatusFailed:    runnerv1.JobStatus_JOB_STATUS_FAILED,
	JobStatusCanceled:  runnerv1.JobStatus_JOB_STATUS_CANCELED,
}

func JobStatusFromProto(s runnerv1.JobStatus) (string, error) {
	switch s {
	case runnerv1.JobStatus_JOB_STATUS_SUCCEEDED:
		return JobStatusSucceeded, nil
	case runnerv1.JobStatus_JOB_STATUS_FAILED:
		return JobStatusFailed, nil
	}
	return "", fmt.Errorf("reported job status must be SUCCEEDED or FAILED (got %s)", s)
}

func (j *Job) Validate() error {
	if j.RunnerId == uuid.Nil {
		return fmt.Errorf("runner_id is required")
	}
	if j.RevisionId == uuid.Nil {
		return fmt.Errorf("revision_id is required")
	}
	if j.RunId == uuid.Nil {
		return fmt.Errorf("run_id is required")
	}
	switch j.JobType {
	case JobTypePlan, JobTypeApply, JobTypeDestroyPlan, JobTypeDestroyApply:
	case "":
		return fmt.Errorf("job_type is required")
	default:
		return fmt.Errorf("invalid job_type: %s", j.JobType)
	}
	switch j.Status {
	case JobStatusPending, JobStatusAssigned, JobStatusRunning,
		JobStatusSucceeded, JobStatusFailed, JobStatusCanceled:
	case "":
		return fmt.Errorf("status is required")
	default:
		return fmt.Errorf("invalid status: %s", j.Status)
	}
	return nil
}

type Job struct {
	Id                  uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	RunnerId            uuid.UUID  `gorm:"type:uuid;not null;index:idx_jobs_runner_status,priority:1"`
	RevisionId          uuid.UUID  `gorm:"type:uuid;not null;index"`
	RunId               uuid.UUID  `gorm:"type:uuid;not null;index"`
	JobType             string     `gorm:"not null"`
	Status              string     `gorm:"not null;index:idx_jobs_runner_status,priority:2"`
	ClaimedAt           *time.Time `gorm:"column:claimed_at"`
	ClaimedByInstanceId *uuid.UUID `gorm:"type:uuid;column:claimed_by_instance_id"`
	CreatedAt           time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
}

func (j *Job) ToProto() *runnerv1.Job {
	proto := &runnerv1.Job{
		Id:         j.Id.String(),
		RunnerId:   j.RunnerId.String(),
		RevisionId: j.RevisionId.String(),
		RunId:      j.RunId.String(),
		JobType:    jobTypeToProto[j.JobType],
		Status:     jobStatusToProto[j.Status],
		CreatedAt:  timestamppb.New(j.CreatedAt),
	}
	if j.StartedAt != nil {
		proto.StartedAt = timestamppb.New(*j.StartedAt)
	}
	if j.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*j.CompletedAt)
	}
	return proto
}
