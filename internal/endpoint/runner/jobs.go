package runner

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
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
		BackendConfig:    a.buildBackendConfig(rev.ComponentId, dep.EnvironmentId),
		TerraformVersion: "",
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

	jobStatus, err := model.JobStatusFromProto(result.GetStatus())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	now := time.Now()

	if _, err := a.jobStore.Update(ctx, job, map[string]any{
		"status":       jobStatus,
		"completed_at": now,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update job: %v", err)
	}

	rev, err := a.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load revision: %v", err)
	}

	revFields := model.DeriveRevisionUpdate(job.JobType, jobStatus, result)
	revFields["completed_at"] = now

	// Persist plan output to object storage.
	if planOutput := result.GetPlanOutput(); planOutput != "" {
		key := fmt.Sprintf("plans/%s/plan.txt", rev.Id)
		if err := a.objStore.PutObject(ctx, a.objBucket, key, []byte(planOutput)); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to write plan output to object storage: %v", err)
		}
		revFields["plan_output_key"] = key
	}

	if _, err := a.revisionStore.Update(ctx, rev, revFields); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update revision: %v", err)
	}

	// Capture Terraform outputs after a successful apply.
	dep, err := a.deploymentStore.Get(ctx, rev.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load deployment: %v", err)
	}

	if jobStatus == model.JobStatusSucceeded {
		if err := a.captureOutputs(ctx, job, rev, dep, result); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to capture outputs: %v", err)
		}

		// After a successful apply, cancel stale AWAITING_APPROVAL revisions
		// for the same (component, environment) in other deployments. Their
		// plans were computed against old state and are no longer valid.
		if job.JobType == model.JobTypeApply || job.JobType == model.JobTypeDestroyApply {
			canceled, err := a.revisionStore.CancelStaleAwaitingApproval(
				ctx, rev.ComponentId, dep.EnvironmentId, dep.Id)
			if err != nil {
				a.logger.Error("state invalidation: failed to cancel stale revisions",
					zap.String("component_id", rev.ComponentId.String()),
					zap.String("environment_id", dep.EnvironmentId.String()),
					zap.Error(err))
			} else if canceled > 0 {
				a.logger.Info("state invalidation: canceled stale awaiting-approval revisions",
					zap.String("component_id", rev.ComponentId.String()),
					zap.String("environment_id", dep.EnvironmentId.String()),
					zap.Int64("count", canceled))
			}
		}

		// Promote blocked jobs whose dependencies are now satisfied.
		if err := a.promoteUnblockedJobs(ctx, rev); err != nil {
			a.logger.Error("failed to promote unblocked jobs",
				zap.String("deployment_id", rev.DeploymentId.String()),
				zap.String("component", rev.ComponentName),
				zap.Error(err))
		}
	}

	// Recompute deployment composite status from the fresh revision set.
	revisions, err := a.revisionStore.ListByDeployment(ctx, rev.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list revisions: %v", err)
	}
	depStatus := model.DeriveDeploymentStatus(revisions)
	depFields := map[string]any{
		"status":     depStatus,
		"updated_at": now,
	}
	if model.IsTerminalDeploymentStatus(depStatus) {
		depFields["completed_at"] = now
	}
	if _, err := a.deploymentStore.Update(ctx, dep, depFields); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update deployment: %v", err)
	}

	// When a deployment reaches a terminal state, promote the next queued
	// deployment for the same (app, env).
	if model.IsTerminalDeploymentStatus(depStatus) {
		a.promoteQueuedDeployment(ctx, dep.ApplicationId, dep.EnvironmentId)
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
