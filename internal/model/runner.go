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
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

const (
	HeartbeatInterval = 30 * time.Second
	HeartbeatTTL      = 3 * HeartbeatInterval
)

type ActiveJobInfo struct {
	JobId     string    `json:"job_id"`
	Phase     string    `json:"phase"`
	StartedAt time.Time `json:"started_at"`
}

func (a ActiveJobInfo) ToProto() *runnerv1.ActiveJobInfo {
	return &runnerv1.ActiveJobInfo{
		JobId:     a.JobId,
		Phase:     runnerv1.JobPhase(runnerv1.JobPhase_value[a.Phase]),
		StartedAt: timestamppb.New(a.StartedAt),
	}
}

func ActiveJobInfoFromProto(p *runnerv1.ActiveJobInfo) ActiveJobInfo {
	info := ActiveJobInfo{
		JobId: p.GetJobId(),
		Phase: p.GetPhase().String(),
	}
	if p.GetStartedAt() != nil {
		info.StartedAt = p.GetStartedAt().AsTime()
	}
	return info
}

type RunnerStatus struct {
	Version           string          `json:"version,omitempty"`
	ActiveJobs        int32           `json:"active_jobs"`
	MaxConcurrentJobs int32           `json:"max_concurrent_jobs"`
	ActiveJobDetails  []ActiveJobInfo `json:"active_job_details,omitempty"`
}

func (s RunnerStatus) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runner status: %w", err)
	}
	return string(b), nil
}

func (s *RunnerStatus) Scan(value any) error {
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
		return fmt.Errorf("cannot scan %T into RunnerStatus", value)
	}
	return json.Unmarshal(b, s)
}

func (s *RunnerStatus) ToProto() *runnerv1.RunnerStatus {
	if s == nil {
		return nil
	}
	proto := &runnerv1.RunnerStatus{
		Version:           s.Version,
		ActiveJobs:        s.ActiveJobs,
		MaxConcurrentJobs: s.MaxConcurrentJobs,
	}
	for i := range s.ActiveJobDetails {
		proto.ActiveJobDetails = append(proto.ActiveJobDetails, s.ActiveJobDetails[i].ToProto())
	}
	return proto
}

func RunnerStatusFromProto(p *runnerv1.RunnerStatus) *RunnerStatus {
	if p == nil {
		return nil
	}
	s := &RunnerStatus{
		Version:           p.GetVersion(),
		ActiveJobs:        p.GetActiveJobs(),
		MaxConcurrentJobs: p.GetMaxConcurrentJobs(),
	}
	for _, j := range p.GetActiveJobDetails() {
		s.ActiveJobDetails = append(s.ActiveJobDetails, ActiveJobInfoFromProto(j))
	}
	return s
}

type Runner struct {
	Id              uuid.UUID     `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name            string        `gorm:"uniqueIndex;not null"`
	Description     string        `gorm:"type:text"`
	Labels          Labels        `gorm:"type:jsonb;default:'{}'"`
	LastHeartbeatAt *time.Time    `gorm:"column:last_heartbeat_at"`
	LastStatus      *RunnerStatus `gorm:"type:jsonb;column:last_status"`
	LastInstanceId  *uuid.UUID    `gorm:"column:last_instance_id;type:uuid"`
	CreatedBy       string        `gorm:"not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
	CreatedByName   string         `gorm:"->;column:created_by_name"`
	CreatedByEmail  string         `gorm:"->;column:created_by_email"`
}

func (r *Runner) ToProto() *runnerv1.Runner {
	return &runnerv1.Runner{
		Id:           r.Id.String(),
		Name:         r.Name,
		Description:  r.Description,
		Labels:       r.Labels,
		HealthStatus: DeriveHealthStatus(r.LastHeartbeatAt, time.Now()),
		CreatedBy:    &commonv1.ActorRef{Id: r.CreatedBy, DisplayName: r.CreatedByName, Email: r.CreatedByEmail},
		CreatedAt:    timestamppb.New(r.CreatedAt),
		UpdatedAt:    timestamppb.New(r.UpdatedAt),
	}
}

func DeriveHealthStatus(lastHeartbeatAt *time.Time, now time.Time) runnerv1.RunnerHealthStatus {
	if lastHeartbeatAt == nil {
		return runnerv1.RunnerHealthStatus_RUNNER_HEALTH_STATUS_PENDING
	}
	if now.Sub(*lastHeartbeatAt) >= HeartbeatTTL {
		return runnerv1.RunnerHealthStatus_RUNNER_HEALTH_STATUS_UNREACHABLE
	}
	return runnerv1.RunnerHealthStatus_RUNNER_HEALTH_STATUS_HEALTHY
}
