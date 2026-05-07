package orchestration

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/model"
	admtemplate "go.admiral.io/admiral/internal/template"
)

// DiffChangeSet computes the structural diff for a change set: per-entry
// deltas relative to current env HEAD, per-variable deltas relative to current
// env USER variables, and the deployed components whose values_template
// references a name touched by this change set.
//
// Available for change sets in any status; for non-OPEN change sets the result
// reflects the diff against the env's CURRENT HEAD, not the env state at the
// time of deploy.
func (s *Service) DiffChangeSet(ctx context.Context, ident string) (*model.ChangeSetDiff, error) {
	cs, err := s.changeSetStore.GetByIdentifier(ctx, ident)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	entries, err := s.changeSetStore.ListEntries(ctx, cs.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list entries: %v", err)
	}
	varEntries, err := s.changeSetStore.ListVariableEntries(ctx, cs.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list variable entries: %v", err)
	}

	deployed, err := s.componentStore.ListByApplicationEnv(ctx, cs.ApplicationId, cs.EnvironmentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list deployed components: %v", err)
	}
	deployedByName := make(map[string]*model.Component, len(deployed))
	for i := range deployed {
		deployedByName[deployed[i].Name] = &deployed[i]
	}

	envVars, err := s.variableStore.Resolve(ctx, cs.ApplicationId, cs.EnvironmentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve env variables: %v", err)
	}
	userVarsByKey := make(map[string]*model.Variable, len(envVars))
	for i := range envVars {
		v := &envVars[i]
		if v.Source != model.VariableSourceUser {
			continue
		}
		userVarsByKey[v.Key] = v
	}

	diff := &model.ChangeSetDiff{}

	for i := range entries {
		ed, err := s.diffEntry(ctx, &entries[i], deployedByName)
		if err != nil {
			return nil, err
		}
		diff.Entries = append(diff.Entries, ed)
	}
	sort.Slice(diff.Entries, func(i, j int) bool {
		return diff.Entries[i].ComponentName < diff.Entries[j].ComponentName
	})

	for i := range varEntries {
		vd, ok := diffVariable(&varEntries[i], userVarsByKey)
		if !ok {
			continue
		}
		diff.Variables = append(diff.Variables, vd)
	}
	sort.Slice(diff.Variables, func(i, j int) bool {
		return diff.Variables[i].Key < diff.Variables[j].Key
	})

	diff.Downstream = computeDownstream(entries, deployed)

	return diff, nil
}

// diffEntry builds the per-entry delta. For UPDATE/DESTROY/ORPHAN it compares
// against the env's current HEAD component (looked up by name); for CREATE
// there is no "old" side and the entry's patch fields are surfaced as the new
// values.
func (s *Service) diffEntry(
	ctx context.Context,
	e *model.ChangeSetEntry,
	deployedByName map[string]*model.Component,
) (model.EntryDiff, error) {
	out := model.EntryDiff{
		ComponentName: e.ComponentName,
		ChangeType:    e.ChangeType,
	}

	switch e.ChangeType {
	case model.ChangeSetEntryTypeCreate:
		out.Module = s.buildModuleDiff(ctx, nil, "", e.ModuleId, derefStr(e.Version))
		if e.ValuesTemplate != nil && *e.ValuesTemplate != "" {
			newV := *e.ValuesTemplate
			out.Values = append(out.Values, model.ValueDiff{
				Key:        "values_template",
				ChangeType: model.DiffChangeTypeAdded,
				New:        &newV,
			})
		}
		if len(e.DependsOn) > 0 {
			out.DependsOnAdded = append(out.DependsOnAdded, e.DependsOn...)
			sort.Strings(out.DependsOnAdded)
		}
		if e.Description != nil {
			v := *e.Description
			out.DescriptionNew = &v
		}

	case model.ChangeSetEntryTypeUpdate:
		head := deployedByName[e.ComponentName]
		if head == nil {
			return out, status.Errorf(codes.FailedPrecondition,
				"UPDATE entry %q references a component that does not exist in this environment", e.ComponentName)
		}

		var oldModID *uuid.UUID
		if head.ModuleId != uuid.Nil {
			id := head.ModuleId
			oldModID = &id
		}
		oldVer := head.Version

		newModID := oldModID
		if e.ModuleId != nil {
			newModID = e.ModuleId
		}
		newVer := oldVer
		if e.Version != nil {
			newVer = *e.Version
		}
		// Only surface a module diff when the entry actually intends to change
		// one of the two fields; otherwise oldX==newX silences buildModuleDiff
		// already, but explicit gating keeps semantics readable.
		if e.ModuleId != nil || e.Version != nil {
			out.Module = s.buildModuleDiff(ctx, oldModID, oldVer, newModID, newVer)
		}

		if e.ValuesTemplate != nil && *e.ValuesTemplate != head.ValuesTemplate {
			oldV := head.ValuesTemplate
			newV := *e.ValuesTemplate
			vd := model.ValueDiff{Key: "values_template"}
			switch {
			case oldV == "" && newV != "":
				vd.ChangeType = model.DiffChangeTypeAdded
				vd.New = &newV
			case oldV != "" && newV == "":
				vd.ChangeType = model.DiffChangeTypeRemoved
				vd.Old = &oldV
			default:
				vd.ChangeType = model.DiffChangeTypeChanged
				vd.Old = &oldV
				vd.New = &newV
			}
			out.Values = append(out.Values, vd)
		}

		if e.DependsOn != nil {
			added, removed := diffStringSlices([]string(head.DependsOn), []string(e.DependsOn))
			out.DependsOnAdded = added
			out.DependsOnRemoved = removed
		}

		if e.Description != nil && *e.Description != head.Description {
			oldD := head.Description
			newD := *e.Description
			out.DescriptionOld = &oldD
			out.DescriptionNew = &newD
		}

	case model.ChangeSetEntryTypeDestroy, model.ChangeSetEntryTypeOrphan:
		// No patch fields; surface nothing beyond the change type. The
		// reviewer reads the name + change_type and knows the component is
		// going away.

	default:
		return out, status.Errorf(codes.Internal, "unknown change_type %q on entry %q", e.ChangeType, e.ComponentName)
	}

	return out, nil
}

// buildModuleDiff resolves module names for old/new IDs and returns a diff
// only when at least one half differs. Returns nil when both halves match.
// Module-name lookup is best-effort: a missing module shows up as id-only on
// the affected half, which is preferable to failing the whole diff request.
func (s *Service) buildModuleDiff(
	ctx context.Context,
	oldID *uuid.UUID, oldVer string,
	newID *uuid.UUID, newVer string,
) *model.ModuleVersionDiff {
	idEqual := (oldID == nil && newID == nil) ||
		(oldID != nil && newID != nil && *oldID == *newID)
	if idEqual && oldVer == newVer {
		return nil
	}
	out := &model.ModuleVersionDiff{}
	if oldID != nil {
		idStr := oldID.String()
		out.ModuleIdOld = &idStr
		if mod, err := s.moduleStore.Get(ctx, *oldID); err == nil {
			n := mod.Name
			out.ModuleNameOld = &n
		}
	}
	if newID != nil {
		idStr := newID.String()
		out.ModuleIdNew = &idStr
		if mod, err := s.moduleStore.Get(ctx, *newID); err == nil {
			n := mod.Name
			out.ModuleNameNew = &n
		}
	}
	if oldVer != "" {
		v := oldVer
		out.VersionOld = &v
	}
	if newVer != "" {
		v := newVer
		out.VersionNew = &v
	}
	return out
}

// diffVariable computes the per-key diff for one change-set variable entry.
// Returns ok=false when the entry would be a no-op (tombstone for a key that
// doesn't exist; or set-to-existing-identical-value).
func diffVariable(
	ve *model.ChangeSetVariableEntry,
	envByKey map[string]*model.Variable,
) (model.VariableDiff, bool) {
	out := model.VariableDiff{Key: ve.Key, Sensitive: ve.Sensitive}
	current := envByKey[ve.Key]

	if ve.IsDelete() {
		if current == nil {
			return out, false
		}
		out.ChangeType = model.DiffChangeTypeRemoved
		if current.Sensitive {
			out.Sensitive = true
		} else {
			oldV := current.Value
			out.Old = &oldV
		}
		return out, true
	}

	newV := ""
	if ve.Value != nil {
		newV = *ve.Value
	}
	if current == nil {
		out.ChangeType = model.DiffChangeTypeAdded
		if !out.Sensitive {
			v := newV
			out.New = &v
		}
		return out, true
	}

	if current.Value == newV && current.Type == ve.Type && current.Sensitive == ve.Sensitive {
		return out, false
	}
	out.ChangeType = model.DiffChangeTypeChanged
	if current.Sensitive || ve.Sensitive {
		out.Sensitive = true
	} else {
		oldV := current.Value
		newVar := newV
		out.Old = &oldV
		out.New = &newVar
	}
	return out, true
}

// computeDownstream surfaces deployed components NOT directly in the change
// set whose values_template references a name touched by the change set.
// Static graph walk; no plan is run.
func computeDownstream(entries []model.ChangeSetEntry, deployed []model.Component) []model.DownstreamImpact {
	if len(entries) == 0 || len(deployed) == 0 {
		return nil
	}

	touched := make(map[string]bool, len(entries))
	for i := range entries {
		touched[entries[i].ComponentName] = true
	}

	out := make([]model.DownstreamImpact, 0)
	for i := range deployed {
		comp := &deployed[i]
		if touched[comp.Name] {
			continue
		}
		if comp.DesiredState != model.ComponentDesiredStateActive {
			continue
		}
		refs := admtemplate.ExtractReferencedComponents(comp.ValuesTemplate)
		var hits []string
		for _, name := range refs {
			if touched[name] {
				hits = append(hits, name)
			}
		}
		if len(hits) == 0 {
			continue
		}
		out = append(out, model.DownstreamImpact{
			ComponentName: comp.Name,
			AffectedBy:    hits,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ComponentName < out[j].ComponentName
	})
	return out
}

// diffStringSlices returns (added, removed) sorted lists for sliceB relative
// to sliceA. Both inputs are treated as sets.
func diffStringSlices(a, b []string) ([]string, []string) {
	setA := make(map[string]bool, len(a))
	for _, s := range a {
		setA[s] = true
	}
	setB := make(map[string]bool, len(b))
	for _, s := range b {
		setB[s] = true
	}
	var added, removed []string
	for s := range setB {
		if !setA[s] {
			added = append(added, s)
		}
	}
	for s := range setA {
		if !setB[s] {
			removed = append(removed, s)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
