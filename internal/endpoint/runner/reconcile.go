package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/model"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

// captureOutputs persists Terraform outputs as infrastructure variables after
// a successful apply, or deletes them after a successful destroy.
func (a *api) captureOutputs(
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
		vars := model.VariablesFromTerraformOutputs(
			outputs,
			rev.ComponentName,
			dep.ApplicationId,
			dep.EnvironmentId,
			"system:output-capture",
		)
		if err := a.variableStore.UpsertInfraOutputs(
			ctx, dep.ApplicationId, dep.EnvironmentId,
			rev.ComponentName, vars,
		); err != nil {
			a.logger.Error("output capture: upsert failed",
				zap.String("component", rev.ComponentName),
				zap.String("deployment_id", dep.Id.String()),
				zap.Error(err))
			return err
		}
		a.logger.Info("output capture: stored infrastructure outputs",
			zap.String("component", rev.ComponentName),
			zap.Int("count", len(vars)))

	case model.JobTypeDestroyApply:
		if err := a.variableStore.DeleteInfraOutputs(
			ctx, dep.ApplicationId, dep.EnvironmentId,
			rev.ComponentName,
		); err != nil {
			a.logger.Error("output capture: delete failed",
				zap.String("component", rev.ComponentName),
				zap.String("deployment_id", dep.Id.String()),
				zap.Error(err))
			return err
		}
		a.logger.Info("output capture: cleared infrastructure outputs after destroy",
			zap.String("component", rev.ComponentName))
	}
	return nil
}

// promoteUnblockedJobs checks for PENDING jobs in the same deployment whose
// BlockedBy dependencies are now all satisfied (their revisions have reached
// a post-plan or post-apply terminal state). Satisfied jobs are promoted from
// PENDING to ASSIGNED so the runner can claim them.
func (a *api) promoteUnblockedJobs(ctx context.Context, completedRev *model.Revision) error {
	pendingJobs, err := a.jobStore.ListByDeploymentAndStatus(ctx, completedRev.DeploymentId, model.JobStatusPending)
	if err != nil {
		return fmt.Errorf("list pending jobs: %w", err)
	}
	if len(pendingJobs) == 0 {
		return nil
	}

	// Load all revisions to build a component-name → status lookup.
	revisions, err := a.revisionStore.ListByDeployment(ctx, completedRev.DeploymentId)
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
			if err := a.jobStore.PromoteToAssigned(ctx, pj.Id); err != nil {
				a.logger.Error("failed to promote job",
					zap.String("job_id", pj.Id.String()),
					zap.Error(err))
				continue
			}
			a.logger.Info("promoted blocked job",
				zap.String("job_id", pj.Id.String()),
				zap.String("component", rev.ComponentName))
		}
	}
	return nil
}

// promoteQueuedDeployment checks for the next queued deployment for the given
// (app, env) and activates it by creating plan jobs.
func (a *api) promoteQueuedDeployment(ctx context.Context, appID, envID uuid.UUID) {
	next, err := a.deploymentStore.FindOldestQueued(ctx, appID, envID)
	if err != nil {
		a.logger.Error("queue promotion: failed to find queued deployment", zap.Error(err))
		return
	}
	if next == nil {
		return
	}

	revisions, err := a.revisionStore.ListByDeployment(ctx, next.Id)
	if err != nil {
		a.logger.Error("queue promotion: failed to list revisions",
			zap.String("deployment_id", next.Id.String()), zap.Error(err))
		return
	}

	env, err := a.envStore.Get(ctx, next.EnvironmentId)
	if err != nil {
		a.logger.Error("queue promotion: failed to load environment",
			zap.String("deployment_id", next.Id.String()), zap.Error(err))
		return
	}
	runnerID, err := env.TerraformRunnerID()
	if err != nil {
		a.logger.Error("queue promotion: environment has no runner",
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
		if _, err := a.jobStore.Create(ctx, job); err != nil {
			a.logger.Error("queue promotion: failed to create job",
				zap.String("deployment_id", next.Id.String()), zap.Error(err))
			return
		}
	}

	if _, err := a.deploymentStore.Update(ctx, next, map[string]any{
		"status":     model.DeploymentStatusRunning,
		"updated_at": time.Now(),
	}); err != nil {
		a.logger.Error("queue promotion: failed to update deployment status",
			zap.String("deployment_id", next.Id.String()), zap.Error(err))
		return
	}

	a.logger.Info("queue promotion: activated queued deployment",
		zap.String("deployment_id", next.Id.String()))
}

// buildBackendConfig generates the Terraform HTTP backend HCL block pointing
// at Admiral's state endpoint. The password is not baked into the HCL; the
// runner passes it via the TF_HTTP_PASSWORD environment variable.
func (a *api) buildBackendConfig(componentID, environmentID uuid.UUID) string {
	if a.baseURL == "" {
		return ""
	}
	stateURL := fmt.Sprintf("%s/api/v1/state/%s/env/%s", a.baseURL, componentID, environmentID)
	lockURL := stateURL + "/lock"
	unlockURL := lockURL
	return fmt.Sprintf(`terraform {
  backend "http" {
    address        = %q
    lock_address   = %q
    unlock_address = %q
    username       = "admiral"
  }
}
`, stateURL, lockURL, unlockURL)
}
