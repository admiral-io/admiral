package environment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
	environmentv1 "go.admiral.io/sdk/proto/admiral/environment/v1"
	variablev1 "go.admiral.io/sdk/proto/admiral/variable/v1"
)

const Name = "endpoint.environment"

var filterColumns = []string{"name", "application_id"}

var variableFilterColumns = []string{"key", "sensitive", "type", "source"}

type api struct {
	store     *store.EnvironmentStore
	varStore  *store.VariableStore
	compStore *store.ComponentStore
	revStore  *store.RevisionStore
	modStore  *store.ModuleStore
	qb        querybuilder.QueryBuilder
	varQb     querybuilder.QueryBuilder
	logger    *zap.Logger
	scope     tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}

	envStore, err := store.NewEnvironmentStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	varStore, err := store.NewVariableStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	compStore, err := store.NewComponentStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	revStore, err := store.NewRevisionStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	modStore, err := store.NewModuleStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	return &api{
		store:     envStore,
		varStore:  varStore,
		compStore: compStore,
		revStore:  revStore,
		modStore:  modStore,
		logger:    log.Named(Name),
		scope:     scope.SubScope("environment"),
		qb:        querybuilder.New("environments", filterColumns),
		varQb:     querybuilder.New("variables", variableFilterColumns),
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
		Environment: env.ToProto(false),
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

	pending, err := a.store.HasPendingChanges(ctx, env.ApplicationId, env.Id)
	if err != nil {
		a.logger.Warn("failed to compute pending changes", zap.String("environment_id", id.String()), zap.Error(err))
	}

	return &environmentv1.GetEnvironmentResponse{
		Environment: env.ToProto(pending),
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
		pending, err := a.store.HasPendingChanges(ctx, envs[i].ApplicationId, envs[i].Id)
		if err != nil {
			a.logger.Warn("failed to compute pending changes", zap.String("environment_id", envs[i].Id.String()), zap.Error(err))
		}
		resp.Environments = append(resp.Environments, envs[i].ToProto(pending))
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
		Environment: env.ToProto(false),
	}, nil
}

func (a *api) DeleteEnvironment(ctx context.Context, req *environmentv1.DeleteEnvironmentRequest) (*environmentv1.DeleteEnvironmentResponse, error) {
	id, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	result, err := a.store.Delete(ctx, id, req.GetForce())
	if err != nil {
		if depErr, ok := errors.AsType[*store.DependentsError](err); ok {
			return nil, status.Errorf(codes.FailedPrecondition, "%s", depErr.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to delete environment: %v", err)
	}

	if result.Runs > 0 {
		a.logger.Info("force-deleted environment",
			zap.String("environment_id", id.String()),
			zap.Int64("runs_deleted", result.Runs),
		)
	}

	return &environmentv1.DeleteEnvironmentResponse{}, nil
}

func (a *api) ListEnvironmentVariables(ctx context.Context, req *environmentv1.ListEnvironmentVariablesRequest) (*environmentv1.ListEnvironmentVariablesResponse, error) {
	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	if _, err := a.store.Get(ctx, envID); err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", envID)
	}

	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	scopeEnv := func(db *gorm.DB) *gorm.DB {
		return db.Where("variables.environment_id = ?", envID)
	}

	vars, err := a.varStore.List(ctx, scopeEnv, a.varQb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list environment variables: %v", err)
	}

	resp := &environmentv1.ListEnvironmentVariablesResponse{
		Variables: make([]*variablev1.Variable, 0, len(vars)),
	}
	for i := range vars {
		resp.Variables = append(resp.Variables, vars[i].ToProto())
	}

	if len(vars) > 0 && int32(len(vars)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := vars[len(vars)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) ListEnvironmentComponents(ctx context.Context, req *environmentv1.ListEnvironmentComponentsRequest) (*environmentv1.ListEnvironmentComponentsResponse, error) {
	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	env, err := a.store.Get(ctx, envID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", envID)
	}

	comps, err := a.compStore.ListByApplicationEnv(ctx, env.ApplicationId, env.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list components: %v", err)
	}

	out := make([]*environmentv1.EnvironmentComponent, 0, len(comps))
	for i := range comps {
		c := &comps[i]
		if c.DesiredState != model.ComponentDesiredStateActive {
			continue
		}
		ec := &environmentv1.EnvironmentComponent{
			Name:         c.Name,
			DesiredState: model.ComponentDesiredStateToProto(c.DesiredState),
			ModuleId:     c.ModuleId.String(),
			Version:      c.Version,
		}
		// Resolve module name + type. Best-effort: a deleted module
		// shouldn't fail the whole list.
		if mod, err := a.modStore.Get(ctx, c.ModuleId); err == nil {
			ec.ModuleName = mod.Name
			if proto, ok := model.ModuleTypeToProto(mod.Type); ok {
				ec.ModuleType = proto
			}
		}
		// Last-succeeded revision for the (component, env) pair. nil when
		// the component has never reached SUCCEEDED.
		rev, err := a.revStore.LastDeployed(ctx, c.Id, env.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "last deployed for %q: %v", c.Name, err)
		}
		if rev != nil {
			ec.LastRevisionId = rev.Id.String()
			ec.LastRevisionStatus = model.RevisionStatusToProto(rev.Status)
			if rev.CompletedAt != nil {
				ec.LastDeployedAt = timestamppb.New(*rev.CompletedAt)
			}
		}
		out = append(out, ec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return &environmentv1.ListEnvironmentComponentsResponse{Components: out}, nil
}
