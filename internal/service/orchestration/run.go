package orchestration

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/dag"
	"go.admiral.io/admiral/internal/model"
	admtemplate "go.admiral.io/admiral/internal/template"
)

// CreateRunParams are the inputs to CreateRun. Callers (endpoint, future
// schedulers) parse and validate their own request shape, then construct
// this struct. The service treats every field as opaque.
type CreateRunParams struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	TriggeredBy   string
	Message       string
	SourceRunID   *uuid.UUID
	ChangeSetID   *uuid.UUID
}

// CreateRun resolves the deployable component set for an application+env,
// snapshots it as revisions, and dispatches plan jobs to the env's runner.
// When SourceRunID is set, revisions are seeded from a prior run's snapshots
// (rollback). When ChangeSetID is set, the component set and variable overlay
// are sourced from the change set entries; otherwise every infra component
// for the env is included (drift correction).
func (s *Service) CreateRun(ctx context.Context, params CreateRunParams) (*model.Run, error) {
	if params.SourceRunID != nil && params.ChangeSetID != nil {
		return nil, status.Error(codes.InvalidArgument, "source_run_id and change_set_id are mutually exclusive")
	}

	if _, err := s.appStore.Get(ctx, params.ApplicationID); err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", params.ApplicationID)
	}

	env, err := s.envStore.Get(ctx, params.EnvironmentID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", params.EnvironmentID)
	}
	if env.ApplicationId != params.ApplicationID {
		return nil, status.Errorf(codes.InvalidArgument, "environment %s does not belong to application %s", params.EnvironmentID, params.ApplicationID)
	}

	runnerID, err := env.TerraformRunnerID()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}
	if _, err := s.runnerStore.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "assigned runner not found: %s", runnerID)
	}

	sourceRevByComponent, err := s.resolveSourceRevisions(ctx, params)
	if err != nil {
		return nil, err
	}

	infraComponents, destroySlugs, changeSetVars, err := s.resolveDeployableComponents(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(infraComponents) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "no components to deploy for this environment")
	}

	// Validate that variable resolution succeeds before persisting the run;
	// the merged map is rebuilt per-revision at render time (renderRevision).
	if _, err := s.resolveVariables(ctx, params.ApplicationID, params.EnvironmentID, changeSetVars); err != nil {
		return nil, err
	}

	phases, slugToComp, blockers, err := buildComponentDAG(infraComponents)
	if err != nil {
		return nil, err
	}

	active, err := s.runStore.FindActive(ctx, params.ApplicationID, params.EnvironmentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check active runs: %v", err)
	}
	queued := active != nil

	initialStatus := model.RunStatusPlanning
	if queued {
		initialStatus = model.RunStatusQueued
	}

	run := &model.Run{
		ApplicationId: params.ApplicationID,
		EnvironmentId: params.EnvironmentID,
		Status:        initialStatus,
		TriggeredBy:   params.TriggeredBy,
		Message:       params.Message,
		SourceRunId:   params.SourceRunID,
		ChangeSetId:   params.ChangeSetID,
	}
	run, err = s.runStore.Create(ctx, run)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create run: %v", err)
	}

	if err := s.snapshotRevisions(ctx, run, phases, slugToComp, sourceRevByComponent, destroySlugs, blockers); err != nil {
		return nil, err
	}

	// Jobs are dispatched only for non-queued runs; queued runs get their
	// jobs created when promoted (see promoteQueuedRun).
	if !queued {
		if err := s.createPlanJobs(ctx, run); err != nil {
			return nil, err
		}
	}

	return run, nil
}

// resolveSourceRevisions loads the source run's revisions for the rollback
// path, returning a slug→revision map. Returns nil when SourceRunID is unset.
func (s *Service) resolveSourceRevisions(ctx context.Context, params CreateRunParams) (map[string]*model.Revision, error) {
	if params.SourceRunID == nil {
		return nil, nil
	}
	srcRun, err := s.runStore.Get(ctx, *params.SourceRunID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "source run not found: %s", *params.SourceRunID)
	}
	if srcRun.ApplicationId != params.ApplicationID {
		return nil, status.Errorf(codes.InvalidArgument, "source run belongs to a different application")
	}
	srcRevisions, err := s.revisionStore.ListByRun(ctx, *params.SourceRunID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load source run revisions: %v", err)
	}
	out := make(map[string]*model.Revision, len(srcRevisions))
	for i := range srcRevisions {
		out[srcRevisions[i].ComponentSlug] = &srcRevisions[i]
	}
	return out, nil
}

// resolveDeployableComponents picks the component set for the run. With a
// change set, it materializes the entries, supersedes any prior non-applying
// run on the same change set, and returns the changeset's variable overlay.
// Without a change set, it lists the env's active infra components (drift
// correction). destroySlugs is non-empty only on the change-set path.
func (s *Service) resolveDeployableComponents(
	ctx context.Context,
	params CreateRunParams,
) ([]model.Component, map[string]bool, []model.ChangeSetVariableEntry, error) {
	if params.ChangeSetID == nil {
		infra, err := s.listEnvironmentInfraComponents(ctx, params.ApplicationID, params.EnvironmentID)
		if err != nil {
			return nil, nil, nil, err
		}
		return infra, nil, nil, nil
	}

	cs, err := s.changeSetStore.Get(ctx, *params.ChangeSetID)
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.NotFound, "change set not found: %s", *params.ChangeSetID)
	}
	if cs.Status != model.ChangeSetStatusOpen {
		return nil, nil, nil, status.Errorf(codes.FailedPrecondition, "change set %s is %s and cannot be deployed", cs.DisplayId, cs.Status)
	}
	if cs.ApplicationId != params.ApplicationID || cs.EnvironmentId != params.EnvironmentID {
		return nil, nil, nil, status.Error(codes.InvalidArgument, "change set's application/environment does not match the run target")
	}

	// Replan policy: auto-supersede a prior active run on the same change
	// set unless it's APPLYING (mid-flight; superseding would leave state
	// incoherent).
	if existing, err := s.runStore.FindActiveByChangeSet(ctx, *params.ChangeSetID); err != nil {
		return nil, nil, nil, status.Errorf(codes.Internal, "failed to check for active run on change set %s: %v", cs.DisplayId, err)
	} else if existing != nil {
		if existing.Status == model.RunStatusApplying {
			return nil, nil, nil, status.Errorf(codes.FailedPrecondition,
				"change set has an active applying run %s; wait for it to complete or cancel it",
				existing.Id)
		}
		if err := s.SupersedeRun(ctx, existing); err != nil {
			return nil, nil, nil, status.Errorf(codes.Internal, "failed to supersede prior run: %v", err)
		}
	}

	entries, err := s.changeSetStore.ListEntries(ctx, *params.ChangeSetID)
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.Internal, "failed to load change set %s entries: %v", cs.DisplayId, err)
	}
	csVars, err := s.changeSetStore.ListVariableEntries(ctx, *params.ChangeSetID)
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.Internal, "failed to load change set %s variable entries: %v", cs.DisplayId, err)
	}
	if err := s.checkChangeSetConflicts(ctx, cs, entries); err != nil {
		return nil, nil, nil, err
	}
	// Variables-only change set: re-plan every infra component so a variable
	// change reaches everything that consumes it.
	if len(entries) == 0 {
		infra, err := s.listEnvironmentInfraComponents(ctx, params.ApplicationID, params.EnvironmentID)
		if err != nil {
			return nil, nil, nil, err
		}
		return infra, nil, csVars, nil
	}
	infra, destroySlugs, err := s.materializeChangeSetEntries(ctx, cs, params.ApplicationID, params.EnvironmentID, entries)
	if err != nil {
		return nil, nil, nil, err
	}
	return infra, destroySlugs, csVars, nil
}

// resolveVariables resolves the env's variable map and applies the change
// set's variable overlay (delete tombstones drop keys; non-tombstones
// upsert). The returned map is the Var slot of the template eval context.
func (s *Service) resolveVariables(
	ctx context.Context,
	appID, envID uuid.UUID,
	csVars []model.ChangeSetVariableEntry,
) (map[string]any, error) {
	resolved, err := s.variableStore.Resolve(ctx, appID, envID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resolve variables: %v", err)
	}
	out := make(map[string]any, len(resolved)+len(csVars))
	for _, v := range resolved {
		out[v.Key] = v.Value
	}
	for i := range csVars {
		v := &csVars[i]
		if v.IsDelete() {
			delete(out, v.Key)
			continue
		}
		out[v.Key] = *v.Value
	}
	return out, nil
}

// buildComponentDAG topologically sorts the component set. Edges come from
// each component's DependsOn (slug-keyed) plus the slugs referenced by its
// values_template (output references). Returns the topo phases, a slug→component
// index, and a slug→blockers index used to populate Revision.BlockedBy.
func buildComponentDAG(infra []model.Component) ([][]string, map[string]*model.Component, map[string][]string, error) {
	slugToComp := make(map[string]*model.Component, len(infra))
	for i := range infra {
		c := &infra[i]
		slugToComp[c.Slug] = c
	}

	g := dag.New()
	for _, comp := range infra {
		g.AddNode(comp.Slug)
		for _, depSlug := range comp.DependsOn {
			if _, ok := slugToComp[depSlug]; ok {
				g.AddEdge(comp.Slug, depSlug)
			}
		}
		for _, depSlug := range admtemplate.ExtractOutputSlugs(comp.ValuesTemplate) {
			if _, ok := slugToComp[depSlug]; ok {
				g.AddEdge(comp.Slug, depSlug)
			}
		}
	}

	phases, err := g.TopoSort()
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "component dependency error: %v", err)
	}

	blockers := make(map[string][]string, len(infra))
	for slug := range slugToComp {
		blockers[slug] = g.Dependencies(slug)
	}
	return phases, slugToComp, blockers, nil
}

// snapshotRevisions persists one revision per component in topological order.
// For rollback runs (sourceRevByComponent populated), the revision is seeded
// from the prior run's stored fields verbatim. Otherwise it pulls module/version
// from the component's HEAD and stores the raw values_template; rendering is
// deferred until each plan job is dispatched (renderRevision), so cross-component
// {{ .component.<slug>.<output> }} references resolve against the upstream's
// captured outputs instead of an empty context. change_type is derived vs the
// env's last-deployed revision (CREATE/UPDATE/RECREATE/NO_CHANGE), unless the
// change set marked the slug for DESTROY.
func (s *Service) snapshotRevisions(
	ctx context.Context,
	run *model.Run,
	phases [][]string,
	slugToComp map[string]*model.Component,
	sourceRevByComponent map[string]*model.Revision,
	destroySlugs map[string]bool,
	blockers map[string][]string,
) error {
	for _, phase := range phases {
		for _, compSlug := range phase {
			comp := slugToComp[compSlug]

			var (
				moduleId         uuid.UUID
				sourceId         *uuid.UUID
				version          string
				valuesTemplate   string
				resolvedValues   string
				workingDirectory string
			)

			if srcRev, ok := sourceRevByComponent[compSlug]; ok {
				moduleId = srcRev.ModuleId
				sourceId = srcRev.SourceId
				version = srcRev.Version
				valuesTemplate = srcRev.ValuesTemplate
				resolvedValues = srcRev.ResolvedValues
				workingDirectory = srcRev.WorkingDirectory
			} else {
				mod, err := s.moduleStore.Get(ctx, comp.ModuleId)
				if err != nil {
					return status.Errorf(codes.Internal, "failed to resolve module for component %s: %v", comp.Id, err)
				}
				moduleId = comp.ModuleId
				sourceId = &mod.SourceId
				version = comp.Version
				valuesTemplate = comp.ValuesTemplate
				workingDirectory = filepath.Join(mod.Root, mod.Path)
			}

			prevRev, err := s.revisionStore.LastDeployed(ctx, comp.Id, run.EnvironmentId)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to query previous revision for %q: %v", comp.Slug, err)
			}
			changeType := model.RevisionChangeTypeCreate
			var previousRevisionId *uuid.UUID
			if prevRev != nil {
				previousRevisionId = &prevRev.Id
				switch {
				case moduleId != prevRev.ModuleId:
					changeType = model.RevisionChangeTypeRecreate
				case version != prevRev.Version || valuesTemplate != prevRev.ValuesTemplate:
					changeType = model.RevisionChangeTypeUpdate
				default:
					changeType = model.RevisionChangeTypeNoChange
				}
			}
			if destroySlugs[compSlug] {
				changeType = model.RevisionChangeTypeDestroy
			}

			rev := &model.Revision{
				RunId:              run.Id,
				ComponentId:        comp.Id,
				ComponentSlug:      comp.Slug,
				Kind:               comp.Kind,
				Status:             model.RevisionStatusQueued,
				ChangeType:         changeType,
				PreviousRevisionId: previousRevisionId,
				ModuleId:           moduleId,
				SourceId:           sourceId,
				Version:            version,
				ValuesTemplate:     valuesTemplate,
				ResolvedValues:     resolvedValues,
				DependsOn:          pq.StringArray(comp.DependsOn),
				BlockedBy:          pq.StringArray(blockers[compSlug]),
				WorkingDirectory:   workingDirectory,
			}
			if _, err := s.revisionStore.Create(ctx, rev); err != nil {
				return status.Errorf(codes.Internal, "failed to create revision: %v", err)
			}
		}
	}
	return nil
}

// ApplyRun transitions a run from plan phase to apply phase by dispatching
// APPLY jobs for every revision currently in AWAITING_APPROVAL. Conflicts
// are re-checked here -- HEAD may have moved between plan and apply even
// though it was clean at run create time.
func (s *Service) ApplyRun(ctx context.Context, runID uuid.UUID) (*model.Run, error) {
	run, err := s.runStore.Get(ctx, runID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "run not found: %s", runID)
	}

	pending, err := s.revisionStore.ListByRunAndStatus(ctx, runID, model.RevisionStatusAwaitingApproval)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query revisions: %v", err)
	}
	if len(pending) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "run has no revisions awaiting approval")
	}

	if run.ChangeSetId != nil {
		cs, err := s.changeSetStore.Get(ctx, *run.ChangeSetId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to load change set: %v", err)
		}
		entries, err := s.changeSetStore.ListEntries(ctx, *run.ChangeSetId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to load change set %s entries: %v", cs.DisplayId, err)
		}
		if err := s.checkChangeSetConflicts(ctx, cs, entries); err != nil {
			return nil, err
		}
	}

	env, err := s.envStore.Get(ctx, run.EnvironmentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load environment: %v", err)
	}
	runnerID, err := env.TerraformRunnerID()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	// Apply jobs that depend on already-SUCCEEDED siblings start ASSIGNED so
	// the runner can claim them immediately; otherwise nothing would ever fire
	// promoteUnblockedJobs (which only runs on a peer's CompleteJob). This
	// matters when a downstream component plans only after its upstream's apply
	// has already finished (the b1 gate-stiffening flow).
	allRevs, err := s.revisionStore.ListByRun(ctx, runID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list revisions: %v", err)
	}
	revBySlug := make(map[string]*model.Revision, len(allRevs))
	for i := range allRevs {
		revBySlug[allRevs[i].ComponentSlug] = &allRevs[i]
	}

	for i := range pending {
		rev := pending[i]
		if _, err := s.revisionStore.Update(ctx, &rev, map[string]any{
			"status": model.RevisionStatusApplying,
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update revision status: %v", err)
		}
		jobStatus := model.JobStatusAssigned
		for _, blockerSlug := range rev.BlockedBy {
			blocker, ok := revBySlug[blockerSlug]
			if !ok || !model.IsRevisionSatisfiedFor(model.JobTypeApply, blocker.Status, false) {
				jobStatus = model.JobStatusPending
				break
			}
		}
		job := &model.Job{
			RunnerId:   runnerID,
			RevisionId: rev.Id,
			RunId:      run.Id,
			JobType:    model.JobTypeApply,
			Status:     jobStatus,
		}
		if _, err := s.jobStore.Create(ctx, job); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create apply job: %v", err)
		}
	}

	if _, err := s.runStore.Update(ctx, run, map[string]any{
		"status":     model.RunStatusApplying,
		"updated_at": time.Now(),
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update run: %v", err)
	}

	run, err = s.runStore.Get(ctx, runID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to reload run: %v", err)
	}
	return run, nil
}

// CancelRun marks an in-progress run, plus its non-terminal revisions and
// jobs, as CANCELED and advances the run queue.
func (s *Service) CancelRun(ctx context.Context, runID uuid.UUID) (*model.Run, error) {
	run, err := s.runStore.Get(ctx, runID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "run not found: %s", runID)
	}
	if model.IsTerminalRunStatus(run.Status) {
		return nil, status.Errorf(codes.FailedPrecondition, "run is already in terminal status: %s", run.Status)
	}

	if err := s.revisionStore.CancelNonTerminal(ctx, runID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cancel revisions: %v", err)
	}
	if err := s.jobStore.CancelNonTerminal(ctx, runID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cancel jobs: %v", err)
	}

	now := time.Now()
	run, err = s.runStore.Update(ctx, run, map[string]any{
		"status":       model.RunStatusCanceled,
		"updated_at":   now,
		"completed_at": now,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update run: %v", err)
	}

	s.promoteQueuedRun(ctx, run.ApplicationId, run.EnvironmentId)
	return run, nil
}

// SupersedeRun marks a run and its non-terminal revisions/jobs as SUPERSEDED.
// Used when a newer plan lands for the same change set or when the change
// set is edited. The transition is system-driven; CANCELED is reserved for
// user-initiated cancels.
//
// The caller must ensure the run is NOT APPLYING -- mid-apply supersede would
// leave state incoherent. Endpoints check that pre-call so they can phrase
// the rejection in their own message.
func (s *Service) SupersedeRun(ctx context.Context, run *model.Run) error {
	now := time.Now()
	if err := s.revisionStore.SupersedeNonTerminal(ctx, run.Id); err != nil {
		return fmt.Errorf("supersede revisions: %w", err)
	}
	// Jobs have no SUPERSEDED status; cancel them. The supersede semantics
	// live on the run + revision rows.
	if err := s.jobStore.CancelNonTerminal(ctx, run.Id); err != nil {
		return fmt.Errorf("supersede jobs: %w", err)
	}
	if _, err := s.runStore.Update(ctx, run, map[string]any{
		"status":       model.RunStatusSuperseded,
		"updated_at":   now,
		"completed_at": now,
	}); err != nil {
		return fmt.Errorf("supersede run: %w", err)
	}
	return nil
}

// promoteQueuedRun checks for the next queued run for the given (app, env)
// and activates it by creating plan jobs.
func (s *Service) promoteQueuedRun(ctx context.Context, appID, envID uuid.UUID) {
	next, err := s.runStore.FindOldestQueued(ctx, appID, envID)
	if err != nil {
		s.logger.Error("queue promotion: failed to find queued run", zap.Error(err))
		return
	}
	if next == nil {
		return
	}

	if err := s.createPlanJobs(ctx, next); err != nil {
		s.logger.Error("queue promotion: failed to create jobs",
			zap.String("run_id", next.Id.String()), zap.Error(err))
		return
	}

	if _, err := s.runStore.Update(ctx, next, map[string]any{
		"status":     model.RunStatusPlanning,
		"updated_at": time.Now(),
	}); err != nil {
		s.logger.Error("queue promotion: failed to update run status",
			zap.String("run_id", next.Id.String()), zap.Error(err))
		return
	}

	s.logger.Info("queue promotion: activated queued run",
		zap.String("run_id", next.Id.String()))
}

// createPlanJobs dispatches PLAN jobs for every revision on a run. Jobs
// whose revision has no blockers start ASSIGNED so the runner can claim
// them immediately; jobs with blockers start PENDING and are promoted by
// promoteUnblockedJobs once their dependencies finish.
func (s *Service) createPlanJobs(ctx context.Context, run *model.Run) error {
	revisions, err := s.revisionStore.ListByRun(ctx, run.Id)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to list revisions: %v", err)
	}
	env, err := s.envStore.Get(ctx, run.EnvironmentId)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to load environment: %v", err)
	}
	runnerID, err := env.TerraformRunnerID()
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	for i := range revisions {
		rev := &revisions[i]
		jobStatus := model.JobStatusPending
		if len(rev.BlockedBy) == 0 {
			// Render now so the runner reads the rendered ResolvedValues
			// when it claims the job. Blocked revisions render later in
			// promoteUnblockedJobs once their dependencies finish.
			if err := s.renderRevision(ctx, run, rev); err != nil {
				return status.Errorf(codes.InvalidArgument, "%v", err)
			}
			jobStatus = model.JobStatusAssigned
		}
		job := &model.Job{
			RunnerId:   runnerID,
			RevisionId: rev.Id,
			RunId:      run.Id,
			JobType:    model.JobTypePlan,
			Status:     jobStatus,
		}
		if _, err := s.jobStore.Create(ctx, job); err != nil {
			return status.Errorf(codes.Internal, "failed to create plan job: %v", err)
		}
	}
	return nil
}
