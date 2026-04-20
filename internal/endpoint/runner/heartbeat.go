package runner

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/model"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

func (a *api) Heartbeat(ctx context.Context, req *runnerv1.HeartbeatRequest) (*runnerv1.HeartbeatResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	if req.GetStatus() == nil {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	instanceID, err := uuid.Parse(req.GetInstanceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid instance_id: %v", err)
	}

	snapshot := model.RunnerStatusFromProto(req.GetStatus())

	if err := a.store.UpdateHeartbeat(ctx, runnerID, snapshot, instanceID, time.Now()); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record heartbeat: %v", err)
	}

	return &runnerv1.HeartbeatResponse{
		Ack:                  true,
		NextHeartbeatSeconds: int32(model.HeartbeatInterval.Seconds()),
	}, nil
}
