package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"go.admiral.io/admiral/internal/model"
	admtemplate "go.admiral.io/admiral/internal/template"
)

// renderRevision evaluates rev.ValuesTemplate against the current variable
// state for (run.app, run.env) plus any change-set variable overlay, and
// persists the rendered string as rev.ResolvedValues. Called immediately
// before a plan job is dispatched so cross-component {{ .component.<name>.* }}
// references resolve against upstream outputs that have already been captured
// to the variable store. Safe to retry: re-render produces the same result
// given stable inputs.
//
// Rollback runs (run.SourceRunId set) carry the source revision's already-
// rendered ResolvedValues verbatim; re-rendering would defeat the "replay the
// same inputs" guarantee, so render is a no-op for them.
func (s *Service) renderRevision(ctx context.Context, run *model.Run, rev *model.Revision) error {
	if run.SourceRunId != nil {
		return nil
	}
	if strings.TrimSpace(rev.ValuesTemplate) == "" {
		return nil
	}

	app, err := s.appStore.Get(ctx, run.ApplicationId)
	if err != nil {
		return fmt.Errorf("load application: %w", err)
	}
	env, err := s.envStore.Get(ctx, run.EnvironmentId)
	if err != nil {
		return fmt.Errorf("load environment: %w", err)
	}

	var csVars []model.ChangeSetVariableEntry
	if run.ChangeSetId != nil {
		csVars, err = s.changeSetStore.ListVariableEntries(ctx, *run.ChangeSetId)
		if err != nil {
			return fmt.Errorf("load change set variable entries: %w", err)
		}
	}
	varMap, err := s.resolveVariables(ctx, run.ApplicationId, run.EnvironmentId, csVars)
	if err != nil {
		return err
	}

	compMap, err := s.resolveComponentOutputs(ctx, run.ApplicationId, run.EnvironmentId)
	if err != nil {
		return err
	}

	evalCtx := &admtemplate.EvalContext{
		Var:       varMap,
		Component: compMap,
		App:       admtemplate.AppMeta{Name: app.Name, Id: run.ApplicationId.String()},
		Env:       admtemplate.EnvMeta{Name: env.Name, Id: run.EnvironmentId.String()},
		Run:       admtemplate.RunMeta{Id: run.Id.String()},
		Self:      admtemplate.SelfMeta{Name: rev.ComponentName},
	}

	rendered, err := admtemplate.Evaluate(rev.ValuesTemplate, evalCtx)
	if err != nil {
		return fmt.Errorf("evaluate values_template for component %q: %w", rev.ComponentName, err)
	}

	if _, err := s.revisionStore.Update(ctx, rev, map[string]any{
		"resolved_values": rendered,
	}); err != nil {
		return fmt.Errorf("persist resolved_values for component %q: %w", rev.ComponentName, err)
	}
	rev.ResolvedValues = rendered
	return nil
}

// resolveComponentOutputs groups infrastructure-source variables for (app, env)
// into the {{ .component.<name>.<output> }} namespace. Keys are stored as
// "<component_name>.<output_name>" by captureOutputs after a successful apply;
// this is the inverse mapping consumed by the template engine.
func (s *Service) resolveComponentOutputs(ctx context.Context, appID, envID uuid.UUID) (map[string]map[string]any, error) {
	vars, err := s.variableStore.Resolve(ctx, appID, envID)
	if err != nil {
		return nil, fmt.Errorf("resolve component outputs: %w", err)
	}
	out := make(map[string]map[string]any)
	for i := range vars {
		v := &vars[i]
		if v.Source != model.VariableSourceInfrastructure {
			continue
		}
		compName, outputName, ok := strings.Cut(v.Key, ".")
		if !ok || compName == "" || outputName == "" {
			continue
		}
		if out[compName] == nil {
			out[compName] = map[string]any{}
		}
		out[compName][outputName] = v.Value
	}
	return out, nil
}
