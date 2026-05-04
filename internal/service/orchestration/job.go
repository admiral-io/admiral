package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/model"
	admtemplate "go.admiral.io/admiral/internal/template"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

// CompleteJob processes a runner's job result report. It updates the job and
// revision status, persists plan output, captures infrastructure outputs,
// promotes blocked jobs, recomputes run status, and advances the run queue
// when a run reaches a terminal state.
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
	run, err := s.runStore.Get(ctx, rev.RunId)
	if err != nil {
		return fmt.Errorf("load run: %w", err)
	}

	if jobStatus == model.JobStatusSucceeded {
		if err := s.captureOutputs(ctx, job, rev, run, result); err != nil {
			return fmt.Errorf("capture outputs: %w", err)
		}

		// After a successful apply, cancel stale AWAITING_APPROVAL revisions
		// for the same (component, environment) in other runs. Their plans
		// were computed against old state and are no longer valid.
		if job.JobType == model.JobTypeApply || job.JobType == model.JobTypeDestroyApply {
			canceled, err := s.revisionStore.CancelStaleAwaitingApproval(
				ctx, rev.ComponentId, run.EnvironmentId, run.Id)
			if err != nil {
				s.logger.Error("state invalidation: failed to cancel stale revisions",
					zap.String("component_id", rev.ComponentId.String()),
					zap.String("environment_id", run.EnvironmentId.String()),
					zap.Error(err))
			} else if canceled > 0 {
				s.logger.Info("state invalidation: canceled stale awaiting-approval revisions",
					zap.String("component_id", rev.ComponentId.String()),
					zap.String("environment_id", run.EnvironmentId.String()),
					zap.Int64("count", canceled))
			}
		}

		// Promote blocked jobs whose dependencies are now satisfied.
		if err := s.promoteUnblockedJobs(ctx, rev); err != nil {
			s.logger.Error("failed to promote unblocked jobs",
				zap.String("run_id", rev.RunId.String()),
				zap.String("component", rev.ComponentSlug),
				zap.Error(err))
		}

		// Per-revision change set reconcile: on apply success, propagate an
		// UPDATE entry's patch to the component's HEAD so subsequent runs
		// see the new desired state. Best-effort -- a failure here logs but
		// does not block the run.
		if run.ChangeSetId != nil && job.JobType == model.JobTypeApply {
			if err := s.reconcileChangeSetRevision(ctx, run, rev); err != nil {
				s.logger.Error("change set: per-revision reconcile failed",
					zap.String("run_id", run.Id.String()),
					zap.String("change_set_id", s.changeSetDisplayID(ctx, *run.ChangeSetId)),
					zap.String("component", rev.ComponentSlug),
					zap.Error(err))
			}
		}
	}

	// 4. Recompute run composite status.
	revisions, err := s.revisionStore.ListByRun(ctx, rev.RunId)
	if err != nil {
		return fmt.Errorf("list revisions: %w", err)
	}
	runStatus := model.DeriveRunStatus(revisions)
	// Snapshot prior terminality before Update mutates run.Status in place
	// (GORM's .Updates copies fields onto the struct).
	wasTerminal := model.IsTerminalRunStatus(run.Status)
	runFields := map[string]any{
		"status":     runStatus,
		"updated_at": now,
	}
	if model.IsTerminalRunStatus(runStatus) {
		runFields["completed_at"] = now
	}
	if _, err := s.runStore.Update(ctx, run, runFields); err != nil {
		return fmt.Errorf("update run: %w", err)
	}

	// 4.5. Per-run-terminal change set finalization. Only fires on the
	// transition into a terminal state and only when the run fully
	// succeeded -- partial failure leaves the change set OPEN for the user to
	// retry. ORPHAN entries, variable entries, and the changeset status flip
	// land here; per-revision reconcile already handled successful UPDATEs.
	if !wasTerminal && runStatus == model.RunStatusSucceeded && run.ChangeSetId != nil {
		if err := s.finalizeChangeSet(ctx, run); err != nil {
			s.logger.Error("change set: finalization failed",
				zap.String("run_id", run.Id.String()),
				zap.String("change_set_id", s.changeSetDisplayID(ctx, *run.ChangeSetId)),
				zap.Error(err))
		}
	}

	// 5. Advance the run queue.
	if model.IsTerminalRunStatus(runStatus) {
		s.promoteQueuedRun(ctx, run.ApplicationId, run.EnvironmentId)
	}

	return nil
}

// captureOutputs persists Terraform outputs as infrastructure variables after
// a successful apply, or deletes them after a successful destroy.
func (s *Service) captureOutputs(
	ctx context.Context,
	job *model.Job,
	rev *model.Revision,
	run *model.Run,
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
			rev.ComponentSlug,
			run.ApplicationId,
			run.EnvironmentId,
			"system:output-capture",
		)
		if err := s.variableStore.UpsertInfraOutputs(
			ctx, run.ApplicationId, run.EnvironmentId,
			rev.ComponentSlug, vars,
		); err != nil {
			s.logger.Error("output capture: upsert failed",
				zap.String("component", rev.ComponentSlug),
				zap.String("run_id", run.Id.String()),
				zap.Error(err))
			return err
		}
		s.logger.Info("output capture: stored infrastructure outputs",
			zap.String("component", rev.ComponentSlug),
			zap.Int("count", len(vars)))

	case model.JobTypeDestroyApply:
		if err := s.variableStore.DeleteInfraOutputs(
			ctx, run.ApplicationId, run.EnvironmentId,
			rev.ComponentSlug,
		); err != nil {
			s.logger.Error("output capture: delete failed",
				zap.String("component", rev.ComponentSlug),
				zap.String("run_id", run.Id.String()),
				zap.Error(err))
			return err
		}
		s.logger.Info("output capture: cleared infrastructure outputs after destroy",
			zap.String("component", rev.ComponentSlug))
	}
	return nil
}

// promoteUnblockedJobs checks for PENDING jobs in the same run whose
// BlockedBy dependencies are now all satisfied. Plan jobs that reference an
// upstream component's outputs (`{{ .component.<slug>.* }}`) require the
// upstream to be SUCCEEDED (apply done, outputs captured) before promotion;
// other plan jobs unblock at AWAITING_APPROVAL. Just before promoting a plan
// job, the revision's values_template is rendered against the latest captured
// outputs; the runner then reads the populated ResolvedValues from the bundle.
func (s *Service) promoteUnblockedJobs(ctx context.Context, completedRev *model.Revision) error {
	pendingJobs, err := s.jobStore.ListByRunAndStatus(ctx, completedRev.RunId, model.JobStatusPending)
	if err != nil {
		return fmt.Errorf("list pending jobs: %w", err)
	}
	if len(pendingJobs) == 0 {
		return nil
	}

	revisions, err := s.revisionStore.ListByRun(ctx, completedRev.RunId)
	if err != nil {
		return fmt.Errorf("list revisions: %w", err)
	}
	revBySlug := make(map[string]*model.Revision, len(revisions))
	revByID := make(map[uuid.UUID]*model.Revision, len(revisions))
	for i := range revisions {
		revBySlug[revisions[i].ComponentSlug] = &revisions[i]
		revByID[revisions[i].Id] = &revisions[i]
	}

	run, err := s.runStore.Get(ctx, completedRev.RunId)
	if err != nil {
		return fmt.Errorf("load run: %w", err)
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

		outputRefBlockers := outputRefSet(rev.ValuesTemplate)
		allSatisfied := true
		for _, blockerName := range rev.BlockedBy {
			blocker, ok := revBySlug[blockerName]
			if !ok {
				allSatisfied = false
				break
			}
			if !model.IsRevisionSatisfiedFor(pj.JobType, blocker.Status, outputRefBlockers[blockerName]) {
				allSatisfied = false
				break
			}
		}

		if !allSatisfied {
			continue
		}

		// Plan jobs render against the just-captured upstream outputs; apply
		// jobs reuse the values rendered during plan (the user approved that
		// specific plan, so the apply must run with the same inputs).
		if pj.JobType == model.JobTypePlan || pj.JobType == model.JobTypeDestroyPlan {
			if err := s.renderRevision(ctx, run, rev); err != nil {
				s.logger.Error("failed to render values_template before promotion",
					zap.String("job_id", pj.Id.String()),
					zap.String("component", rev.ComponentSlug),
					zap.Error(err))
				continue
			}
		}

		if err := s.jobStore.PromoteToAssigned(ctx, pj.Id); err != nil {
			s.logger.Error("failed to promote job",
				zap.String("job_id", pj.Id.String()),
				zap.Error(err))
			continue
		}
		s.logger.Info("promoted blocked job",
			zap.String("job_id", pj.Id.String()),
			zap.String("component", rev.ComponentSlug))
	}
	return nil
}

// outputRefSet returns the set of component slugs whose outputs are referenced
// by the given values_template (via `{{ .component.<slug>.* }}`).
func outputRefSet(valuesTemplate string) map[string]bool {
	slugs := admtemplate.ExtractOutputSlugs(valuesTemplate)
	if len(slugs) == 0 {
		return nil
	}
	out := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		out[s] = true
	}
	return out
}
