package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/store"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

// Service encapsulates the job lifecycle state machine: job completion,
// revision status derivation, output capture, dependency promotion,
// and deployment queue advancement.
type Service struct {
	jobStore        *store.JobStore
	revisionStore   *store.RevisionStore
	deploymentStore *store.DeploymentStore
	envStore        *store.EnvironmentStore
	variableStore   *store.VariableStore
	objStore        objectstorage.Service
	objBucket       string
	baseURL         string
	logger          *zap.Logger
}

func New(
	jobStore *store.JobStore,
	revisionStore *store.RevisionStore,
	deploymentStore *store.DeploymentStore,
	envStore *store.EnvironmentStore,
	variableStore *store.VariableStore,
	objStore objectstorage.Service,
	objBucket string,
	baseURL string,
	logger *zap.Logger,
) *Service {
	return &Service{
		jobStore:        jobStore,
		revisionStore:   revisionStore,
		deploymentStore: deploymentStore,
		envStore:        envStore,
		variableStore:   variableStore,
		objStore:        objStore,
		objBucket:       objBucket,
		baseURL:         baseURL,
		logger:          logger.Named("orchestration"),
	}
}

// CompleteJob processes a runner's job result report. It updates the job and
// revision status, persists plan output, captures infrastructure outputs,
// promotes blocked jobs, recomputes deployment status, and advances the
// deployment queue when a deployment reaches a terminal state.
func (s *Service) CompleteJob(ctx context.Context, job *model.Job, result *runnerv1.JobResult) error {
	jobStatus, err := model.JobStatusFromProto(result.GetStatus())
	if err != nil {
		return fmt.Errorf("invalid job status: %w", err)
	}

	now := time.Now()

	// 1. Mark the job complete.
	if _, err := s.jobStore.Update(ctx, job, map[string]any{
		"status":       jobStatus,
		"completed_at": now,
	}); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	// 2. Derive and apply revision status.
	rev, err := s.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		return fmt.Errorf("load revision: %w", err)
	}

	revFields := model.DeriveRevisionUpdate(job.JobType, jobStatus, result)
	revFields["completed_at"] = now

	// Persist plan output to object storage. On failure, override the
	// revision status to FAILED so we never proceed to apply without a
	// stored plan.
	if planOutput := result.GetPlanOutput(); planOutput != "" {
		key := fmt.Sprintf("plans/%s/plan.txt", rev.Id)
		if err := s.objStore.PutObject(ctx, s.objBucket, key, []byte(planOutput)); err != nil {
			s.logger.Error("failed to persist plan output, marking revision errored",
				zap.String("revision_id", rev.Id.String()),
				zap.Error(err))
			revFields["status"] = model.RevisionStatusFailed
			revFields["error_message"] = fmt.Sprintf("plan output storage failed: %v", err)
		} else {
			revFields["plan_output_key"] = key
		}
	}

	if _, err := s.revisionStore.Update(ctx, rev, revFields); err != nil {
		return fmt.Errorf("update revision: %w", err)
	}

	// 3. Post-success side effects.
	dep, err := s.deploymentStore.Get(ctx, rev.DeploymentId)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}

	if jobStatus == model.JobStatusSucceeded {
		if err := s.captureOutputs(ctx, job, rev, dep, result); err != nil {
			return fmt.Errorf("capture outputs: %w", err)
		}

		// After a successful apply, cancel stale AWAITING_APPROVAL revisions
		// for the same (component, environment) in other deployments. Their
		// plans were computed against old state and are no longer valid.
		if job.JobType == model.JobTypeApply || job.JobType == model.JobTypeDestroyApply {
			canceled, err := s.revisionStore.CancelStaleAwaitingApproval(
				ctx, rev.ComponentId, dep.EnvironmentId, dep.Id)
			if err != nil {
				s.logger.Error("state invalidation: failed to cancel stale revisions",
					zap.String("component_id", rev.ComponentId.String()),
					zap.String("environment_id", dep.EnvironmentId.String()),
					zap.Error(err))
			} else if canceled > 0 {
				s.logger.Info("state invalidation: canceled stale awaiting-approval revisions",
					zap.String("component_id", rev.ComponentId.String()),
					zap.String("environment_id", dep.EnvironmentId.String()),
					zap.Int64("count", canceled))
			}
		}

		// Promote blocked jobs whose dependencies are now satisfied.
		if err := s.promoteUnblockedJobs(ctx, rev); err != nil {
			s.logger.Error("failed to promote unblocked jobs",
				zap.String("deployment_id", rev.DeploymentId.String()),
				zap.String("component", rev.ComponentName),
				zap.Error(err))
		}
	}

	// 4. Recompute deployment composite status.
	revisions, err := s.revisionStore.ListByDeployment(ctx, rev.DeploymentId)
	if err != nil {
		return fmt.Errorf("list revisions: %w", err)
	}
	depStatus := model.DeriveDeploymentStatus(revisions)
	depFields := map[string]any{
		"status":     depStatus,
		"updated_at": now,
	}
	if model.IsTerminalDeploymentStatus(depStatus) {
		depFields["completed_at"] = now
	}
	if _, err := s.deploymentStore.Update(ctx, dep, depFields); err != nil {
		return fmt.Errorf("update deployment: %w", err)
	}

	// 5. Advance the deployment queue.
	if model.IsTerminalDeploymentStatus(depStatus) {
		s.promoteQueuedDeployment(ctx, dep.ApplicationId, dep.EnvironmentId)
	}

	return nil
}

// BuildBackendConfig generates the Terraform HTTP backend HCL block pointing
// at Admiral's state endpoint.
func (s *Service) BuildBackendConfig(componentID, environmentID uuid.UUID) string {
	if s.baseURL == "" {
		return ""
	}
	stateURL := fmt.Sprintf("%s/api/v1/state/%s/env/%s", s.baseURL, componentID, environmentID)
	lockURL := stateURL + "/lock"
	return fmt.Sprintf(`terraform {
  backend "http" {
    address        = %q
    lock_address   = %q
    unlock_address = %q
    username       = "admiral"
  }
}
`, stateURL, lockURL, lockURL)
}

// captureOutputs persists Terraform outputs as infrastructure variables after
// a successful apply, or deletes them after a successful destroy.
func (s *Service) captureOutputs(
	ctx context.Context,
	job *model.Job,
	rev *model.Revision,
	dep *model.Deployment,
	result *runnerv1.JobResult,
) error {
	switch job.JobType {
	case model.JobTypeApply:
		outputs := result.GetOutputs()
		if len(outputs) == 0 {
			return nil
		}
		vars := model.VariablesFromEngineOutputs(
			outputs,
			rev.ComponentName,
			dep.ApplicationId,
			dep.EnvironmentId,
			"system:output-capture",
		)
		if err := s.variableStore.UpsertInfraOutputs(
			ctx, dep.ApplicationId, dep.EnvironmentId,
			rev.ComponentName, vars,
		); err != nil {
			s.logger.Error("output capture: upsert failed",
				zap.String("component", rev.ComponentName),
				zap.String("deployment_id", dep.Id.String()),
				zap.Error(err))
			return err
		}
		s.logger.Info("output capture: stored infrastructure outputs",
			zap.String("component", rev.ComponentName),
			zap.Int("count", len(vars)))

	case model.JobTypeDestroyApply:
		if err := s.variableStore.DeleteInfraOutputs(
			ctx, dep.ApplicationId, dep.EnvironmentId,
			rev.ComponentName,
		); err != nil {
			s.logger.Error("output capture: delete failed",
				zap.String("component", rev.ComponentName),
				zap.String("deployment_id", dep.Id.String()),
				zap.Error(err))
			return err
		}
		s.logger.Info("output capture: cleared infrastructure outputs after destroy",
			zap.String("component", rev.ComponentName))
	}
	return nil
}

// promoteUnblockedJobs checks for PENDING jobs in the same deployment whose
// BlockedBy dependencies are now all satisfied (their revisions have reached
// a post-plan or post-apply terminal state). Satisfied jobs are promoted from
// PENDING to ASSIGNED so the runner can claim them.
func (s *Service) promoteUnblockedJobs(ctx context.Context, completedRev *model.Revision) error {
	pendingJobs, err := s.jobStore.ListByDeploymentAndStatus(ctx, completedRev.DeploymentId, model.JobStatusPending)
	if err != nil {
		return fmt.Errorf("list pending jobs: %w", err)
	}
	if len(pendingJobs) == 0 {
		return nil
	}

	revisions, err := s.revisionStore.ListByDeployment(ctx, completedRev.DeploymentId)
	if err != nil {
		return fmt.Errorf("list revisions: %w", err)
	}
	revByComponent := make(map[string]*model.Revision, len(revisions))
	revByID := make(map[uuid.UUID]*model.Revision, len(revisions))
	for i := range revisions {
		revByComponent[revisions[i].ComponentName] = &revisions[i]
		revByID[revisions[i].Id] = &revisions[i]
	}

	for i := range pendingJobs {
		pj := &pendingJobs[i]
		rev, ok := revByID[pj.RevisionId]
		if !ok {
			continue
		}
		if len(rev.BlockedBy) == 0 {
			continue
		}

		allSatisfied := true
		for _, blockerName := range rev.BlockedBy {
			blocker, ok := revByComponent[blockerName]
			if !ok {
				allSatisfied = false
				break
			}
			if !model.IsRevisionSatisfiedFor(pj.JobType, blocker.Status) {
				allSatisfied = false
				break
			}
		}

		if allSatisfied {
			if err := s.jobStore.PromoteToAssigned(ctx, pj.Id); err != nil {
				s.logger.Error("failed to promote job",
					zap.String("job_id", pj.Id.String()),
					zap.Error(err))
				continue
			}
			s.logger.Info("promoted blocked job",
				zap.String("job_id", pj.Id.String()),
				zap.String("component", rev.ComponentName))
		}
	}
	return nil
}

// promoteQueuedDeployment checks for the next queued deployment for the given
// (app, env) and activates it by creating plan jobs.
func (s *Service) promoteQueuedDeployment(ctx context.Context, appID, envID uuid.UUID) {
	next, err := s.deploymentStore.FindOldestQueued(ctx, appID, envID)
	if err != nil {
		s.logger.Error("queue promotion: failed to find queued deployment", zap.Error(err))
		return
	}
	if next == nil {
		return
	}

	revisions, err := s.revisionStore.ListByDeployment(ctx, next.Id)
	if err != nil {
		s.logger.Error("queue promotion: failed to list revisions",
			zap.String("deployment_id", next.Id.String()), zap.Error(err))
		return
	}

	env, err := s.envStore.Get(ctx, next.EnvironmentId)
	if err != nil {
		s.logger.Error("queue promotion: failed to load environment",
			zap.String("deployment_id", next.Id.String()), zap.Error(err))
		return
	}
	runnerID, err := env.TerraformRunnerID()
	if err != nil {
		s.logger.Error("queue promotion: environment has no runner",
			zap.String("deployment_id", next.Id.String()), zap.Error(err))
		return
	}

	for i := range revisions {
		rev := &revisions[i]
		jobStatus := model.JobStatusPending
		if len(rev.BlockedBy) == 0 {
			jobStatus = model.JobStatusAssigned
		}
		job := &model.Job{
			RunnerId:     runnerID,
			RevisionId:   rev.Id,
			DeploymentId: next.Id,
			JobType:      model.JobTypePlan,
			Status:       jobStatus,
		}
		if _, err := s.jobStore.Create(ctx, job); err != nil {
			s.logger.Error("queue promotion: failed to create job",
				zap.String("deployment_id", next.Id.String()), zap.Error(err))
			return
		}
	}

	if _, err := s.deploymentStore.Update(ctx, next, map[string]any{
		"status":     model.DeploymentStatusRunning,
		"updated_at": time.Now(),
	}); err != nil {
		s.logger.Error("queue promotion: failed to update deployment status",
			zap.String("deployment_id", next.Id.String()), zap.Error(err))
		return
	}

	s.logger.Info("queue promotion: activated queued deployment",
		zap.String("deployment_id", next.Id.String()))
}