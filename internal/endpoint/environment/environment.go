package environment

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
	environmentv1 "go.admiral.io/sdk/proto/admiral/environment/v1"
)

const Name = "endpoint.environment"

var filterColumns = []string{"name", "application_id"}

type api struct {
	store          *store.EnvironmentStore
	componentStore *store.ComponentStore
	overrideStore  *store.ComponentOverrideStore
	revisionStore  *store.RevisionStore
	qb             querybuilder.QueryBuilder
	logger         *zap.Logger
	scope          tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
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
	revisionStore, err := store.NewRevisionStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	return &api{
		store:          envStore,
		componentStore: componentStore,
		overrideStore:  overrideStore,
		revisionStore:  revisionStore,
		logger:         log.Named(Name),
		scope:  scope.SubScope("environment"),
		qb:     querybuilder.New(filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	environmentv1.RegisterEnvironmentAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(environmentv1.RegisterEnvironmentAPIHandler)
}

func (a *api) CreateEnvironment(ctx context.Context, req *environmentv1.CreateEnvironmentRequest) (*environmentv1.CreateEnvironmentResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	env := &model.Environment{
		ApplicationId:         uuid.MustParse(req.GetApplicationId()),
		Name:                  req.GetName(),
		Description:           req.GetDescription(),
		WorkloadTargets:       model.WorkloadTargetsFromProto(req.GetWorkloadTargets()),
		InfrastructureTargets: model.InfrastructureTargetsFromProto(req.GetInfrastructureTargets()),
		Labels:                model.Labels(req.GetLabels()),
		CreatedBy:             claims.Subject,
	}

	env, err = a.store.Create(ctx, env)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create environment: %v", err)
	}

	return &environmentv1.CreateEnvironmentResponse{
		Environment: env.ToProto(),
	}, nil
}

func (a *api) GetEnvironment(ctx context.Context, req *environmentv1.GetEnvironmentRequest) (*environmentv1.GetEnvironmentResponse, error) {
	id, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	env, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", id)
	}

	env.HasPendingChanges = a.computePendingChanges(ctx, env.ApplicationId, env.Id)

	return &environmentv1.GetEnvironmentResponse{
		Environment: env.ToProto(),
	}, nil
}

func (a *api) ListEnvironments(ctx context.Context, req *environmentv1.ListEnvironmentsRequest) (*environmentv1.ListEnvironmentsResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	envs, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list environments: %v", err)
	}

	resp := &environmentv1.ListEnvironmentsResponse{}
	for i := range envs {
		envs[i].HasPendingChanges = a.computePendingChanges(ctx, envs[i].ApplicationId, envs[i].Id)
		resp.Environments = append(resp.Environments, envs[i].ToProto())
	}

	if len(envs) > 0 && int32(len(envs)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := envs[len(envs)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateEnvironment(ctx context.Context, req *environmentv1.UpdateEnvironmentRequest) (*environmentv1.UpdateEnvironmentResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	envProto := req.GetEnvironment()
	if envProto == nil {
		return nil, status.Error(codes.InvalidArgument, "environment is required")
	}

	id, err := uuid.Parse(envProto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	env, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", id)
	}

	fields := map[string]any{
		"updated_at": time.Now(),
	}

	mask := req.GetUpdateMask()
	if mask == nil || len(mask.GetPaths()) == 0 {
		fields["name"] = envProto.GetName()
		fields["description"] = envProto.GetDescription()
		fields["workload_targets"] = model.WorkloadTargetsFromProto(envProto.GetWorkloadTargets())
		fields["infrastructure_targets"] = model.InfrastructureTargetsFromProto(envProto.GetInfrastructureTargets())
		fields["labels"] = model.Labels(envProto.GetLabels())
	} else {
		for _, path := range mask.GetPaths() {
			switch path {
			case "name":
				fields["name"] = envProto.GetName()
			case "description":
				fields["description"] = envProto.GetDescription()
			case "workload_targets":
				fields["workload_targets"] = model.WorkloadTargetsFromProto(envProto.GetWorkloadTargets())
			case "infrastructure_targets":
				fields["infrastructure_targets"] = model.InfrastructureTargetsFromProto(envProto.GetInfrastructureTargets())
			case "labels":
				fields["labels"] = model.Labels(envProto.GetLabels())
			default:
				return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
			}
		}
	}

	env, err = a.store.Update(ctx, env, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update environment: %v", err)
	}

	return &environmentv1.UpdateEnvironmentResponse{
		Environment: env.ToProto(),
	}, nil
}

func (a *api) DeleteEnvironment(ctx context.Context, req *environmentv1.DeleteEnvironmentRequest) (*environmentv1.DeleteEnvironmentResponse, error) {
	id, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", id)
	}

	return &environmentv1.DeleteEnvironmentResponse{}, nil
}

// computePendingChanges returns true if any component in the application has
// configuration that differs from its last successfully deployed revision in
// this environment. Compares module_id, version, and values_template (after
// override merge) against the last SUCCEEDED revision's snapshot.
func (a *api) computePendingChanges(ctx context.Context, applicationID, environmentID uuid.UUID) bool {
	components, err := a.componentStore.ListByApplicationID(ctx, applicationID)
	if err != nil {
		a.logger.Warn("pending changes: failed to list components",
			zap.String("application_id", applicationID.String()),
			zap.Error(err))
		return false
	}
	if len(components) == 0 {
		return false
	}

	overrides, err := a.overrideStore.ListByApplicationEnv(ctx, applicationID, environmentID)
	if err != nil {
		a.logger.Warn("pending changes: failed to list overrides",
			zap.Error(err))
		return false
	}
	overrideMap := make(map[uuid.UUID]*model.ComponentOverride, len(overrides))
	for i := range overrides {
		overrideMap[overrides[i].ComponentId] = &overrides[i]
	}

	lastDeployed, err := a.revisionStore.LastDeployedByAppEnv(ctx, applicationID, environmentID)
	if err != nil {
		a.logger.Warn("pending changes: failed to query last deployed",
			zap.Error(err))
		return false
	}

	// If nothing has ever been deployed, any component means pending changes.
	if len(lastDeployed) == 0 && len(components) > 0 {
		return true
	}

	for i := range components {
		comp := components[i]
		if comp.Kind != model.ComponentKindInfrastructure {
			continue
		}

		// Apply environment override.
		if o, ok := overrideMap[comp.Id]; ok {
			if o.ApplyToModel(&comp) {
				// Component is disabled for this environment. If it was
				// previously deployed, that's a pending change.
				if _, wasDeployed := lastDeployed[comp.Id]; wasDeployed {
					return true
				}
				continue
			}
		}

		rev, ok := lastDeployed[comp.Id]
		if !ok {
			// Component exists but has never been deployed here.
			return true
		}

		// Compare the fields that matter for deployment.
		if comp.ModuleId != rev.ModuleId {
			return true
		}
		if comp.Version != rev.Version {
			return true
		}
		if comp.ValuesTemplate != rev.ResolvedValues {
			return true
		}
	}

	return false
}
