package deployment

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/dag"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/store"
	admtemplate "go.admiral.io/admiral/internal/template"
	deploymentv1 "go.admiral.io/sdk/proto/admiral/deployment/v1"
)

const Name = "endpoint.deployment"

var filterColumns = []string{"application_id", "environment_id", "status", "trigger_type"}

type api struct {
	deploymentv1.UnimplementedDeploymentAPIServer

	deploymentStore *store.DeploymentStore
	revisionStore   *store.RevisionStore
	jobStore        *store.JobStore
	appStore        *store.ApplicationStore
	envStore        *store.EnvironmentStore
	componentStore  *store.ComponentStore
	overrideStore   *store.ComponentOverrideStore
	moduleStore     *store.ModuleStore
	runnerStore     *store.RunnerStore
	variableStore   *store.VariableStore
	objStore        objectstorage.Service
	objBucket       string
	qb              querybuilder.QueryBuilder
	logger          *zap.Logger
	scope           tally.Scope
}

func New(cfg *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	deploymentStore, err := store.NewDeploymentStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	revisionStore, err := store.NewRevisionStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	jobStore, err := store.NewJobStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	appStore, err := store.NewApplicationStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	envStore, err := store.NewEnvironmentStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	componentStore, err := store.NewComponentStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	overrideStore, err := store.NewComponentOverrideStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	moduleStore, err := store.NewModuleStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	runnerStore, err := store.NewRunnerStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	variableStore, err := store.NewVariableStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	objStore, err := service.GetService[objectstorage.Service](objectstorage.Name)
	if err != nil {
		return nil, fmt.Errorf("object storage is required: %w", err)
	}
	objBucket := cfg.Services.ObjectStorage.Bucket

	return &api{
		deploymentStore: deploymentStore,
		revisionStore:   revisionStore,
		jobStore:        jobStore,
		appStore:        appStore,
		envStore:        envStore,
		componentStore:  componentStore,
		overrideStore:   overrideStore,
		moduleStore:     moduleStore,
		runnerStore:     runnerStore,
		variableStore:   variableStore,
		objStore:        objStore,
		objBucket:       objBucket,
		logger:          log.Named(Name),
		scope:           scope.SubScope("deployment"),
		qb:              querybuilder.New(filterColumns),
	}, nil
}

const planOutputRoutePattern = "/api/v1/deployments/{deployment_id}/revisions/{revision_id}/plan"

func (a *api) Register(r endpoint.Registrar) error {
	deploymentv1.RegisterDeploymentAPIServer(r.GRPCServer(), a)
	r.HTTPMux().HandleFunc("GET "+planOutputRoutePattern, a.servePlanOutput)
	return r.RegisterJSONGateway(deploymentv1.RegisterDeploymentAPIHandler)
}

func (a *api) CreateDeployment(ctx context.Context, req *deploymentv1.CreateDeploymentRequest) (*deploymentv1.CreateDeploymentResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	if req.GetDestroy() {
		return nil, status.Error(codes.Unimplemented, "destroy deployments are not yet supported")
	}

	// Parse optional rollback source.
	var sourceDeploymentId *uuid.UUID
	if raw := req.GetSourceDeploymentId(); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid source_deployment_id: %v", err)
		}
		sourceDeploymentId = &id
	}

	appID, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application_id: %v", err)
	}
	app, err := a.appStore.Get(ctx, appID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", appID)
	}

	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment_id: %v", err)
	}
	env, err := a.envStore.Get(ctx, envID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", envID)
	}
	if env.ApplicationId != appID {
		return nil, status.Errorf(codes.InvalidArgument, "environment %s does not belong to application %s", envID, appID)
	}

	runnerID, err := env.TerraformRunnerID()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}
	if _, err := a.runnerStore.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "assigned runner not found: %s", runnerID)
	}

	// If rolling back, load and validate the source deployment's revisions.
	var sourceRevByComponent map[string]*model.Revision
	if sourceDeploymentId != nil {
		srcDep, err := a.deploymentStore.Get(ctx, *sourceDeploymentId)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "source deployment not found: %s", sourceDeploymentId)
		}
		if srcDep.ApplicationId != appID {
			return nil, status.Errorf(codes.InvalidArgument, "source deployment belongs to a different application")
		}
		srcRevisions, err := a.revisionStore.ListByDeployment(ctx, *sourceDeploymentId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to load source deployment revisions: %v", err)
		}
		sourceRevByComponent = make(map[string]*model.Revision, len(srcRevisions))
		for i := range srcRevisions {
			sourceRevByComponent[srcRevisions[i].ComponentName] = &srcRevisions[i]
		}
	}

	components, err := a.componentStore.ListByApplicationID(ctx, appID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list components: %v", err)
	}

	// Load all overrides for this (app, env) in one query.
	overrides, err := a.overrideStore.ListByApplicationEnv(ctx, appID, envID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list component overrides: %v", err)
	}
	overrideMap := make(map[uuid.UUID]*model.ComponentOverride, len(overrides))
	for i := range overrides {
		overrideMap[overrides[i].ComponentId] = &overrides[i]
	}

	// Apply overrides and filter to non-disabled infrastructure components.
	var infraComponents []model.Component
	for i := range components {
		comp := components[i]
		if o, ok := overrideMap[comp.Id]; ok {
			if o.ApplyToModel(&comp) {
				continue // disabled for this environment
			}
		}
		if comp.Kind == model.ComponentKindInfrastructure {
			infraComponents = append(infraComponents, comp)
		}
	}
	if len(infraComponents) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "application has no infrastructure components to deploy for this environment")
	}

	// Resolve the effective variable set (global → app → env merge).
	resolved, err := a.variableStore.Resolve(ctx, appID, envID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resolve variables: %v", err)
	}
	varMap := make(map[string]any, len(resolved))
	for _, v := range resolved {
		varMap[v.Key] = v.Value
	}

	evalCtx := &admtemplate.EvalContext{
		Var: varMap,
		App: admtemplate.AppMeta{
			Name: app.Name,
			Id:   appID.String(),
		},
		Env: admtemplate.EnvMeta{
			Name: env.Name,
			Id:   envID.String(),
		},
		// Component outputs are not yet available (populated after apply).
		// Deploy.Id and Self.Name are set per-revision below.
	}

	// Build dependency graph from explicit DependsOn (UUIDs) and implicit
	// template references ({{ .component.NAME.OUTPUT }}).
	idToName := make(map[string]string, len(infraComponents))
	nameToComp := make(map[string]*model.Component, len(infraComponents))
	for i := range infraComponents {
		c := &infraComponents[i]
		idToName[c.Id.String()] = c.Name
		nameToComp[c.Name] = c
	}

	g := dag.New()
	for _, comp := range infraComponents {
		g.AddNode(comp.Name)

		// Explicit dependencies (stored as component UUIDs).
		for _, depID := range comp.DependsOn {
			if depName, ok := idToName[depID]; ok {
				g.AddEdge(comp.Name, depName)
			}
		}
		// Implicit dependencies from template expressions.
		for _, depName := range admtemplate.ExtractComponentNames(comp.ValuesTemplate) {
			if _, ok := nameToComp[depName]; ok {
				g.AddEdge(comp.Name, depName)
			}
		}
	}

	phases, err := g.TopoSort()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "component dependency error: %v", err)
	}

	// Build a set of phase-0 component names for determining initial job status.
	phase0 := make(map[string]struct{}, len(phases[0]))
	for _, name := range phases[0] {
		phase0[name] = struct{}{}
	}

	triggerType := model.DeploymentTriggerManual
	if claims.Kind == string(authn.TokenKindPAT) {
		triggerType = model.DeploymentTriggerCI
	}

	dep := &model.Deployment{
		ApplicationId:      appID,
		EnvironmentId:      envID,
		Status:             model.DeploymentStatusPending,
		TriggerType:        triggerType,
		TriggeredBy:        claims.Subject,
		Message:            req.GetMessage(),
		SourceDeploymentId: sourceDeploymentId,
	}
	dep, err = a.deploymentStore.Create(ctx, dep)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create deployment: %v", err)
	}
	evalCtx.Deploy = admtemplate.DeployMeta{Id: dep.Id.String()}

	// Create revisions and jobs in topological order.
	for _, phase := range phases {
		for _, compName := range phase {
			comp := nameToComp[compName]
			evalCtx.Self = admtemplate.SelfMeta{Name: comp.Name}

			var (
				moduleId         uuid.UUID
				sourceId         *uuid.UUID
				version          string
				resolvedValues   string
				workingDirectory string
			)

			if srcRev, ok := sourceRevByComponent[compName]; ok {
				// Rollback: use the source revision's snapshot.
				moduleId = srcRev.ModuleId
				sourceId = srcRev.SourceId
				version = srcRev.Version
				resolvedValues = srcRev.ResolvedValues
				workingDirectory = srcRev.WorkingDirectory
			} else {
				// Normal: read from Component HEAD.
				mod, err := a.moduleStore.Get(ctx, comp.ModuleId)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to resolve module for component %s: %v", comp.Id, err)
				}
				moduleId = comp.ModuleId
				sourceId = &mod.SourceId
				version = comp.Version
				workingDirectory = mod.Path

				// Evaluate template expressions in values_template.
				resolvedValues = comp.ValuesTemplate
				if resolvedValues != "" {
					resolvedValues, err = admtemplate.Evaluate(resolvedValues, evalCtx)
					if err != nil {
						return nil, status.Errorf(codes.InvalidArgument,
							"failed to evaluate values_template for component %q: %v", comp.Name, err)
					}
				}
			}

			blockedBy := g.Dependencies(compName)

			rev := &model.Revision{
				DeploymentId:     dep.Id,
				ComponentId:      comp.Id,
				ComponentName:    comp.Name,
				Kind:             comp.Kind,
				Status:           model.RevisionStatusQueued,
				ModuleId:         moduleId,
				SourceId:         sourceId,
				Version:          version,
				ResolvedValues:   resolvedValues,
				DependsOn:        pq.StringArray(comp.DependsOn),
				BlockedBy:        pq.StringArray(blockedBy),
				WorkingDirectory: workingDirectory,
			}
			rev, err = a.revisionStore.Create(ctx, rev)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create revision: %v", err)
			}

			// Phase 0 jobs are immediately claimable; later phases wait for
			// their dependencies to complete.
			jobStatus := model.JobStatusPending
			if _, ok := phase0[compName]; ok {
				jobStatus = model.JobStatusAssigned
			}

			job := &model.Job{
				RunnerId:     runnerID,
				RevisionId:   rev.Id,
				DeploymentId: dep.Id,
				JobType:      model.JobTypePlan,
				Status:       jobStatus,
			}
			if _, err := a.jobStore.Create(ctx, job); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create plan job: %v", err)
			}
		}
	}

	return &deploymentv1.CreateDeploymentResponse{
		Deployment: a.deploymentToProto(ctx, dep),
	}, nil
}

func (a *api) GetDeployment(ctx context.Context, req *deploymentv1.GetDeploymentRequest) (*deploymentv1.GetDeploymentResponse, error) {
	id, err := uuid.Parse(req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment_id: %v", err)
	}
	dep, err := a.deploymentStore.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "deployment not found: %s", id)
	}
	return &deploymentv1.GetDeploymentResponse{
		Deployment: a.deploymentToProto(ctx, dep),
	}, nil
}

func (a *api) ListDeployments(ctx context.Context, req *deploymentv1.ListDeploymentsRequest) (*deploymentv1.ListDeploymentsResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	deployments, err := a.deploymentStore.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list deployments: %v", err)
	}

	resp := &deploymentv1.ListDeploymentsResponse{}
	for i := range deployments {
		resp.Deployments = append(resp.Deployments, deployments[i].ToProto(nil))
	}

	if len(deployments) > 0 && int32(len(deployments)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := deployments[len(deployments)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}
	return resp, nil
}

func (a *api) GetRevision(ctx context.Context, req *deploymentv1.GetRevisionRequest) (*deploymentv1.GetRevisionResponse, error) {
	depID, err := uuid.Parse(req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment_id: %v", err)
	}
	revID, err := uuid.Parse(req.GetRevisionId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid revision_id: %v", err)
	}
	rev, err := a.revisionStore.Get(ctx, revID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "revision not found: %s", revID)
	}
	if rev.DeploymentId != depID {
		return nil, status.Errorf(codes.NotFound, "revision not found: %s", revID)
	}
	return &deploymentv1.GetRevisionResponse{
		Revision: rev.ToProto(),
	}, nil
}

func (a *api) ListRevisions(ctx context.Context, req *deploymentv1.ListRevisionsRequest) (*deploymentv1.ListRevisionsResponse, error) {
	depID, err := uuid.Parse(req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment_id: %v", err)
	}
	revisions, err := a.revisionStore.ListByDeployment(ctx, depID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list revisions: %v", err)
	}
	resp := &deploymentv1.ListRevisionsResponse{}
	for i := range revisions {
		resp.Revisions = append(resp.Revisions, revisions[i].ToProto())
	}
	return resp, nil
}

func (a *api) ApplyDeployment(ctx context.Context, req *deploymentv1.ApplyDeploymentRequest) (*deploymentv1.ApplyDeploymentResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	depID, err := uuid.Parse(req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment_id: %v", err)
	}
	dep, err := a.deploymentStore.Get(ctx, depID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "deployment not found: %s", depID)
	}

	pending, err := a.revisionStore.ListByDeploymentAndStatus(ctx, depID, model.RevisionStatusAwaitingApproval)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query revisions: %v", err)
	}
	if len(pending) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "deployment has no revisions awaiting approval")
	}

	env, err := a.envStore.Get(ctx, dep.EnvironmentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load environment: %v", err)
	}
	runnerID, err := env.TerraformRunnerID()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	for i := range pending {
		rev := pending[i]
		if _, err := a.revisionStore.Update(ctx, &rev, map[string]any{
			"status": model.RevisionStatusApplying,
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update revision status: %v", err)
		}

		// Revisions with no blockers are immediately claimable; others
		// wait for their dependencies to complete (promoted by
		// ReportJobResult).
		jobStatus := model.JobStatusPending
		if len(rev.BlockedBy) == 0 {
			jobStatus = model.JobStatusAssigned
		}

		job := &model.Job{
			RunnerId:     runnerID,
			RevisionId:   rev.Id,
			DeploymentId: dep.Id,
			JobType:      model.JobTypeApply,
			Status:       jobStatus,
		}
		if _, err := a.jobStore.Create(ctx, job); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create apply job: %v", err)
		}
	}

	if _, err := a.deploymentStore.Update(ctx, dep, map[string]any{
		"status":     model.DeploymentStatusRunning,
		"updated_at": time.Now(),
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update deployment: %v", err)
	}

	dep, err = a.deploymentStore.Get(ctx, depID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to reload deployment: %v", err)
	}

	return &deploymentv1.ApplyDeploymentResponse{
		Deployment: a.deploymentToProto(ctx, dep),
	}, nil
}

func (a *api) deploymentToProto(ctx context.Context, dep *model.Deployment) *deploymentv1.Deployment {
	revisions, err := a.revisionStore.ListByDeployment(ctx, dep.Id)
	if err != nil {
		a.logger.Warn("failed to load revisions for summary", zap.String("deployment_id", dep.Id.String()), zap.Error(err))
		return dep.ToProto(nil)
	}

	proto := dep.ToProto(model.DeriveRevisionSummary(revisions))

	// Override with computed composite status so readers see a fresh value
	// until we persist revision-state changes back to the deployment row.
	proto.Status = model.DeploymentStatusToProtoEnum(model.DeriveDeploymentStatus(revisions))
	return proto
}

func (a *api) servePlanOutput(w http.ResponseWriter, r *http.Request) {
	depID, err := uuid.Parse(r.PathValue("deployment_id"))
	if err != nil {
		http.Error(w, "invalid deployment_id", http.StatusBadRequest)
		return
	}
	revID, err := uuid.Parse(r.PathValue("revision_id"))
	if err != nil {
		http.Error(w, "invalid revision_id", http.StatusBadRequest)
		return
	}

	rev, err := a.revisionStore.Get(r.Context(), revID)
	if err != nil {
		http.Error(w, "revision not found", http.StatusNotFound)
		return
	}
	if rev.DeploymentId != depID {
		http.Error(w, "revision not found", http.StatusNotFound)
		return
	}
	if rev.PlanOutputKey == "" {
		http.Error(w, "no plan output available", http.StatusNotFound)
		return
	}
	data, err := a.objStore.GetObject(r.Context(), a.objBucket, rev.PlanOutputKey)
	if err != nil {
		a.logger.Error("failed to read plan output from object storage",
			zap.String("revision_id", revID.String()),
			zap.String("key", rev.PlanOutputKey),
			zap.Error(err))
		http.Error(w, "failed to read plan output", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}
