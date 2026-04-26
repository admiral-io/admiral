package runner

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

const (
	artifactRoutePattern = "/api/v1/runner/jobs/{id}/artifact"
	planFileRoutePattern = "/api/v1/runner/jobs/{id}/plan"
	planFileContentType  = "application/octet-stream"
	maxPlanFileSize      = 256 << 20 // 256 MiB
)

func (a *api) ClaimJob(ctx context.Context, _ *runnerv1.ClaimJobRequest) (*runnerv1.ClaimJobResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	r, err := a.store.Get(ctx, runnerID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	job, err := a.jobStore.ClaimNextJob(ctx, runnerID, r.LastInstanceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to claim job: %v", err)
	}

	resp := &runnerv1.ClaimJobResponse{}
	if job != nil {
		resp.Job = job.ToProto()
	}
	return resp, nil
}

func (a *api) GetJobBundle(ctx context.Context, req *runnerv1.GetJobBundleRequest) (*runnerv1.GetJobBundleResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	jobID, err := uuid.Parse(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job_id: %v", err)
	}

	job, err := a.jobStore.Get(ctx, jobID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", jobID)
	}
	if job.RunnerId != runnerID {
		return nil, status.Errorf(codes.PermissionDenied, "job does not belong to this runner")
	}

	rev, err := a.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load revision: %v", err)
	}

	vars, err := model.ParseResolvedValuesAsVars(rev.ResolvedValues)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse resolved values: %v", err)
	}

	dep, err := a.deploymentStore.Get(ctx, rev.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load deployment: %v", err)
	}

	bundle := &runnerv1.JobBundle{
		ArtifactUrl:      artifactURLForJob(jobID),
		Variables:        vars,
		WorkingDirectory: rev.WorkingDirectory,
		BackendConfig:    a.orchestration.BuildBackendConfig(rev.ComponentId, dep.EnvironmentId),
	}

	// For apply jobs, include the URL to download the binary plan file
	// produced by the preceding plan job.
	if job.JobType == model.JobTypeApply || job.JobType == model.JobTypeDestroyApply {
		if rev.PlanFileKey != "" {
			bundle.PlanFileUrl = planFileURLForJob(jobID)
		}
	}

	return &runnerv1.GetJobBundleResponse{Bundle: bundle}, nil
}

func (a *api) ReportJobResult(ctx context.Context, req *runnerv1.ReportJobResultRequest) (*runnerv1.ReportJobResultResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	jobID, err := uuid.Parse(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job_id: %v", err)
	}

	job, err := a.jobStore.Get(ctx, jobID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", jobID)
	}
	if job.RunnerId != runnerID {
		return nil, status.Errorf(codes.PermissionDenied, "job does not belong to this runner")
	}

	result := req.GetResult()
	if result == nil {
		return nil, status.Error(codes.InvalidArgument, "result is required")
	}

	if err := a.orchestration.CompleteJob(ctx, job, result); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to complete job: %v", err)
	}

	return &runnerv1.ReportJobResultResponse{Ack: true}, nil
}

func (a *api) ListRunnerJobs(ctx context.Context, req *runnerv1.ListRunnerJobsRequest) (*runnerv1.ListRunnerJobsResponse, error) {
	runnerID, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}
	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	jobs, err := a.jobStore.ListByRunner(
		ctx,
		runnerID,
		a.jobsQB.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list jobs: %v", err)
	}

	resp := &runnerv1.ListRunnerJobsResponse{}
	for i := range jobs {
		resp.Jobs = append(resp.Jobs, jobs[i].ToProto())
	}
	if len(jobs) > 0 && int32(len(jobs)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := jobs[len(jobs)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}
	return resp, nil
}

func artifactURLForJob(jobID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/runner/jobs/%s/artifact", jobID)
}

func planFileURLForJob(jobID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/runner/jobs/%s/plan", jobID)
}
