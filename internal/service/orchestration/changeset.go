package orchestration

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/model"
)

// RemoveChangeSetEntry deletes a change-set entry by component name, rolling
// back the speculatively-materialized component when the entry was a CREATE
// that has not produced a SUCCEEDED revision. Without this rollback, removing
// then re-creating a CREATE entry within an OPEN change set fails with a
// non-obvious "component already exists" error from SetEntry's name-collision
// guard.
//
// The change-set state guard (must be OPEN) and any active-run handling are
// the caller's responsibility.
func (s *Service) RemoveChangeSetEntry(ctx context.Context, changeSetID uuid.UUID, name string) error {
	entry, err := s.changeSetStore.GetEntryByName(ctx, changeSetID, name)
	if err != nil {
		// Mirror the prior NotFound shape from DeleteEntryByName.
		return status.Errorf(codes.NotFound, "%v", err)
	}

	// Only CREATE entries with a materialized component can have their
	// underlying component cleaned up here. UPDATE/DESTROY/ORPHAN entries
	// reference an existing deployed component; deleting that component would
	// destroy real state.
	if entry.ChangeType == model.ChangeSetEntryTypeCreate && entry.ComponentId != nil {
		comp, err := s.componentStore.Get(ctx, *entry.ComponentId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Errorf(codes.Internal, "load materialized component: %v", err)
		}
		// A deletion-protected component is never auto-cleaned, even if no
		// SUCCEEDED revision exists yet -- defense-in-depth in case the flag
		// gets set out of band.
		if comp != nil && !comp.DeletionProtection {
			lastDeployed, err := s.revisionStore.LastDeployed(ctx, comp.Id, comp.EnvironmentId)
			if err != nil {
				return status.Errorf(codes.Internal, "check last deployed revision: %v", err)
			}
			if lastDeployed == nil {
				if err := s.componentStore.Delete(ctx, comp.Id); err != nil {
					return status.Errorf(codes.Internal, "delete speculatively-materialized component: %v", err)
				}
			}
		}
	}

	if err := s.changeSetStore.DeleteEntryByName(ctx, changeSetID, name); err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}
	return nil
}

// reconcileChangeSetRevision propagates the change set entry's patch to the
// component's HEAD after the apply revision succeeds. Only UPDATE entries
// touch HEAD here; CREATE was materialized at deploy time, DESTROY clears
// outputs via captureOutputs, and ORPHAN entries don't generate revisions.
//
// The propagation is keyed off the entry's intent (UPDATE) rather than the
// revision's diff-derived change_type so that a syntactic-only template
// change (which resolves to NO_CHANGE on values) still updates HEAD.
func (s *Service) reconcileChangeSetRevision(ctx context.Context, run *model.Run, rev *model.Revision) error {
	entry, err := s.changeSetStore.GetEntryByName(ctx, *run.ChangeSetId, rev.ComponentName)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Missing entry isn't fatal -- e.g., a transitive-dep revision pulled
		// in without a corresponding entry shouldn't update HEAD.
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to load change set entry: %w", err)
	}
	if entry.ChangeType != model.ChangeSetEntryTypeUpdate {
		return nil
	}

	fields := map[string]any{}
	if entry.ModuleId != nil {
		fields["module_id"] = *entry.ModuleId
	}
	if entry.Version != nil {
		fields["version"] = *entry.Version
	}
	if entry.ValuesTemplate != nil {
		fields["values_template"] = *entry.ValuesTemplate
	}
	if entry.DependsOn != nil {
		fields["depends_on"] = entry.DependsOn
	}
	if entry.Description != nil {
		fields["description"] = *entry.Description
	}
	if len(fields) == 0 {
		return nil
	}
	comp, err := s.componentStore.Get(ctx, rev.ComponentId)
	if err != nil {
		return fmt.Errorf("load component: %w", err)
	}
	if _, err := s.componentStore.Update(ctx, comp, fields); err != nil {
		return fmt.Errorf("propagate UPDATE entry to component HEAD: %w", err)
	}
	return nil
}

// finalizeChangeSet runs once when a change set's run fully succeeds. It
// commits the side effects that were deferred during the run:
//
//   - ORPHAN entries: write a Disabled=true component_overrides row for the
//     env so future runs for this (component, env) skip the component.
//     Terraform state is preserved (no destroy was run).
//   - Variable entries: non-tombstones upsert at the env tier; tombstones
//     delete the env-tier row. Other tiers (global, app) are not touched
//     -- when step 2.6 collapses the tier model, env will be the only tier.
//   - Change set: status -> DEPLOYED, run_id -> run.Id.
//
// Partial-failure runs do not call this -- the change set stays OPEN for the
// operator to retry; per-revision UPDATEs already advanced HEAD.
func (s *Service) finalizeChangeSet(ctx context.Context, run *model.Run) error {
	csID := *run.ChangeSetId
	cs, err := s.changeSetStore.Get(ctx, csID)
	if err != nil {
		return fmt.Errorf("load change set: %w", err)
	}

	entries, err := s.changeSetStore.ListEntries(ctx, csID)
	if err != nil {
		return fmt.Errorf("list entries: %w", err)
	}
	for i := range entries {
		e := &entries[i]
		if e.ChangeType != model.ChangeSetEntryTypeOrphan {
			continue
		}
		if e.ComponentId == nil {
			continue
		}
		comp, err := s.componentStore.Get(ctx, *e.ComponentId)
		if err != nil {
			return fmt.Errorf("load component for orphan %q: %w", e.ComponentName, err)
		}
		if _, err := s.componentStore.Update(ctx, comp, map[string]any{
			"desired_state": model.ComponentDesiredStateOrphan,
		}); err != nil {
			return fmt.Errorf("orphan component %q: %w", e.ComponentName, err)
		}
	}

	varEntries, err := s.changeSetStore.ListVariableEntries(ctx, csID)
	if err != nil {
		return fmt.Errorf("list variable entries: %w", err)
	}
	for i := range varEntries {
		ve := &varEntries[i]
		if ve.IsDelete() {
			if err := s.variableStore.DeleteByEnvKey(ctx, run.EnvironmentId, ve.Key); err != nil {
				return fmt.Errorf("delete env variable %q: %w", ve.Key, err)
			}
			continue
		}
		if _, err := s.variableStore.UpsertEnvVariable(
			ctx,
			run.ApplicationId, run.EnvironmentId,
			ve.Key, *ve.Value, ve.Type, ve.Sensitive,
			cs.CreatedBy,
		); err != nil {
			return fmt.Errorf("upsert env variable %q: %w", ve.Key, err)
		}
	}

	if _, err := s.changeSetStore.Update(ctx, cs, map[string]any{
		"status": model.ChangeSetStatusDeployed,
		"run_id": run.Id,
	}); err != nil {
		return fmt.Errorf("mark change set deployed: %w", err)
	}
	return nil
}

// changeSetDisplayID resolves the changeset's display_id for log emission,
// falling back to the UUID when the load fails. Used on the error paths in
// CompleteJob so a logging-side database hiccup doesn't drop the identifier.
func (s *Service) changeSetDisplayID(ctx context.Context, id uuid.UUID) string {
	if cs, err := s.changeSetStore.Get(ctx, id); err == nil {
		return cs.DisplayId
	}
	return id.String()
}

// checkChangeSetConflicts compares each touched entry's base revision (from
// the change set's BaseHeadRevisions snapshot) against the env's current
// last-succeeded revision for that component. A mismatch means another
// changeset advanced HEAD between this changeset's creation and now -- the
// plan is stale and must not apply. Strict reject; the operator discards
// and re-creates the change set to pick up the new HEAD.
//
// Scope is per-entry-component, not env-wide: changes to components NOT
// touched by this changeset don't trigger conflicts. CREATE entries are not
// checked here -- the name-collision check in materializeChangeSetEntries
// already detects the only meaningful conflict shape for them.
func (s *Service) checkChangeSetConflicts(ctx context.Context, cs *model.ChangeSet, entries []model.ChangeSetEntry) error {
	for i := range entries {
		e := &entries[i]
		if e.ChangeType == model.ChangeSetEntryTypeCreate {
			continue
		}
		if e.ComponentId == nil {
			continue
		}
		current, err := s.revisionStore.LastDeployed(ctx, *e.ComponentId, cs.EnvironmentId)
		if err != nil {
			return status.Errorf(codes.Internal, "conflict check: query last deployed for %q: %v", e.ComponentName, err)
		}
		baseRevID, hasBase := cs.BaseHeadRevisions[e.ComponentName]
		var currentRevID uuid.UUID
		if current != nil {
			currentRevID = current.Id
		}
		if !hasBase && current == nil {
			continue
		}
		if hasBase && current != nil && baseRevID == currentRevID {
			continue
		}
		return status.Errorf(codes.FailedPrecondition,
			"change set is stale: component %q moved (base=%s, current=%s); discard and recreate the change set",
			e.ComponentName, baseRevID, currentRevID)
	}
	return nil
}

// listEnvironmentInfraComponents returns the infrastructure components
// currently deployed in (appID, envID). Components are env-scoped, so this
// just lists active infrastructure components for the env. Used by the
// drift-correction path and by variables-only change-set runs (no component
// entries) so a variable change re-plans every component that consumes it.
func (s *Service) listEnvironmentInfraComponents(ctx context.Context, appID, envID uuid.UUID) ([]model.Component, error) {
	components, err := s.componentStore.ListByApplicationEnv(ctx, appID, envID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list components: %v", err)
	}
	out := make([]model.Component, 0, len(components))
	for i := range components {
		comp := components[i]
		if comp.DesiredState != model.ComponentDesiredStateActive {
			continue
		}
		if comp.Kind == model.ComponentKindInfrastructure {
			out = append(out, comp)
		}
	}
	return out, nil
}

// materializeChangeSetEntries builds the deployable component set from a
// change set's entries. Components are env-scoped, so all reads/writes here
// target (appID, envID).
//
//   - CREATE (first attempt): synthesize a fresh components row at the
//     entry's name for this env, then backfill the entry's component_id so
//     a retry can recognize the row. If the run fails the row remains and
//     can be cleaned up via a follow-up DESTROY entry.
//   - CREATE (retry, entry.ComponentId != nil): refresh the existing row's
//     HEAD to the entry's current intent (the user may have edited the entry
//     between attempts), then proceed without erroring on name-already-exists.
//   - UPDATE/DESTROY: load the env-scoped component, apply the entry patch
//     (UPDATE only) in-memory. The components row's HEAD is left untouched
//     until post-run reconciliation.
//   - ORPHAN: skipped -- handled by post-run reconciliation (sets
//     DesiredState=ORPHAN on the component), never produces a runner job.
//
// Returns the deployable components in entry order plus the set of component
// names whose revision must be stamped change_type=DESTROY.
func (s *Service) materializeChangeSetEntries(
	ctx context.Context,
	cs *model.ChangeSet,
	appID, envID uuid.UUID,
	entries []model.ChangeSetEntry,
) ([]model.Component, map[string]bool, error) {
	deployable := make([]model.Component, 0, len(entries))
	destroyNames := map[string]bool{}

	for i := range entries {
		e := &entries[i]
		switch e.ChangeType {
		case model.ChangeSetEntryTypeOrphan:
			continue

		case model.ChangeSetEntryTypeCreate:
			if e.ModuleId == nil {
				return nil, nil, status.Errorf(codes.InvalidArgument, "CREATE entry %q is missing module_id", e.ComponentName)
			}

			var comp *model.Component
			if e.ComponentId != nil {
				existing, err := s.componentStore.Get(ctx, *e.ComponentId)
				if err != nil {
					return nil, nil, status.Errorf(codes.Internal, "failed to load materialized component for retry of CREATE %q: %v", e.ComponentName, err)
				}
				fields := map[string]any{"module_id": *e.ModuleId}
				if e.Version != nil {
					fields["version"] = *e.Version
				}
				if e.ValuesTemplate != nil {
					fields["values_template"] = *e.ValuesTemplate
				}
				if e.DependsOn != nil {
					fields["depends_on"] = pq.StringArray(e.DependsOn)
				}
				if e.Description != nil {
					fields["description"] = *e.Description
				}
				refreshed, err := s.componentStore.Update(ctx, existing, fields)
				if err != nil {
					return nil, nil, status.Errorf(codes.Internal, "failed to refresh component HEAD for retry of CREATE %q: %v", e.ComponentName, err)
				}
				comp = refreshed
			} else {
				if existing, err := s.componentStore.GetByApplicationEnvName(ctx, appID, envID, e.ComponentName); err == nil && existing != nil {
					return nil, nil, status.Errorf(codes.AlreadyExists,
						"change set creates component %q but a component with that name already exists in this environment", e.ComponentName)
				}
				mod, err := s.moduleStore.Get(ctx, *e.ModuleId)
				if err != nil {
					return nil, nil, status.Errorf(codes.InvalidArgument, "module %s for CREATE entry %q not found", *e.ModuleId, e.ComponentName)
				}
				kind := model.DeriveComponentKind(mod.Type)
				if kind == "" {
					return nil, nil, status.Errorf(codes.Internal, "module %s has unrecognized type %q", mod.Id, mod.Type)
				}
				fresh := &model.Component{
					ApplicationId: appID,
					EnvironmentId: envID,
					Name:          e.ComponentName,
					Kind:          kind,
					DesiredState:  model.ComponentDesiredStateActive,
					ModuleId:      *e.ModuleId,
					CreatedBy:     cs.CreatedBy,
				}
				if e.Description != nil {
					fresh.Description = *e.Description
				}
				if e.Version != nil {
					fresh.Version = *e.Version
				}
				if e.ValuesTemplate != nil {
					fresh.ValuesTemplate = *e.ValuesTemplate
				}
				if e.DependsOn != nil {
					fresh.DependsOn = pq.StringArray(e.DependsOn)
				}
				saved, err := s.componentStore.Create(ctx, fresh)
				if err != nil {
					return nil, nil, status.Errorf(codes.Internal, "failed to materialize component for CREATE entry %q: %v", e.ComponentName, err)
				}
				if err := s.changeSetStore.SetEntryComponentID(ctx, e.Id, saved.Id); err != nil {
					return nil, nil, status.Errorf(codes.Internal, "failed to backfill component_id on entry %q: %v", e.ComponentName, err)
				}
				comp = saved
			}

			deployable = append(deployable, *comp)

		case model.ChangeSetEntryTypeUpdate, model.ChangeSetEntryTypeDestroy:
			if e.ComponentId == nil {
				return nil, nil, status.Errorf(codes.InvalidArgument, "%s entry %q is missing component_id", e.ChangeType, e.ComponentName)
			}
			comp, err := s.componentStore.Get(ctx, *e.ComponentId)
			if err != nil {
				return nil, nil, status.Errorf(codes.NotFound, "component for entry %q not found: %v", e.ComponentName, err)
			}
			if comp.ApplicationId != appID || comp.EnvironmentId != envID {
				return nil, nil, status.Errorf(codes.InvalidArgument,
					"component %s for entry %q does not belong to this run's (application, environment)", comp.Id, e.ComponentName)
			}
			if e.ChangeType == model.ChangeSetEntryTypeUpdate {
				if e.ModuleId != nil {
					comp.ModuleId = *e.ModuleId
				}
				if e.Version != nil {
					comp.Version = *e.Version
				}
				if e.ValuesTemplate != nil {
					comp.ValuesTemplate = *e.ValuesTemplate
				}
				if e.DependsOn != nil {
					comp.DependsOn = pq.StringArray(e.DependsOn)
				}
				if e.Description != nil {
					comp.Description = *e.Description
				}
			} else {
				destroyNames[comp.Name] = true
			}
			deployable = append(deployable, *comp)

		default:
			return nil, nil, status.Errorf(codes.InvalidArgument, "unknown change_type %q on entry %q", e.ChangeType, e.ComponentName)
		}
	}
	return deployable, destroyNames, nil
}
