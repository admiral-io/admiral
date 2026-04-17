package component

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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
	componentv1 "go.admiral.io/sdk/proto/admiral/component/v1"
)

const Name = "endpoint.component"

var filterColumns = []string{"name", "kind", "module_id"}

type api struct {
	store     *store.ComponentStore
	overrides *store.ComponentOverrideStore
	modStore  *store.ModuleStore
	appStore  *store.ApplicationStore
	envStore  *store.EnvironmentStore
	qb        querybuilder.QueryBuilder
	logger    *zap.Logger
	scope     tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	compStore, err := store.NewComponentStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	overrideStore, err := store.NewComponentOverrideStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	modStore, err := store.NewModuleStore(db.GormDB())
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

	return &api{
		store:     compStore,
		overrides: overrideStore,
		modStore:  modStore,
		appStore:  appStore,
		envStore:  envStore,
		logger:    log.Named(Name),
		scope:     scope.SubScope("component"),
		qb:        querybuilder.New(filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	componentv1.RegisterComponentAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(componentv1.RegisterComponentAPIHandler)
}

func (a *api) CreateComponent(ctx context.Context, req *componentv1.CreateComponentRequest) (*componentv1.CreateComponentResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	appID, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application ID: %v", err)
	}
	if _, err := a.appStore.Get(ctx, appID); err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", appID)
	}

	moduleID, err := uuid.Parse(req.GetModuleId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid module ID: %v", err)
	}
	mod, err := a.modStore.Get(ctx, moduleID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "module not found: %s", moduleID)
	}

	kind := model.DeriveComponentKind(mod.Type)
	if kind == "" {
		return nil, status.Errorf(codes.Internal, "cannot derive kind from module type: %s", mod.Type)
	}

	if vt := req.GetValuesTemplate(); vt != "" {
		if err := model.ValidateValuesTemplate(vt); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid values_template: %v", err)
		}
	}

	dependsOn, err := model.ParseDependsOn(req.GetDependsOn())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid depends_on: %v", err)
	}

	// Validate depends_on refs belong to same application.
	if err := a.validateDependsOnApp(ctx, dependsOn, appID); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	outputs := model.ComponentOutputsFromProto(req.GetOutputs())
	if kind == model.ComponentKindInfrastructure {
		outputs = nil
	}

	c := &model.Component{
		ApplicationId:  appID,
		Name:           req.GetName(),
		Description:    req.GetDescription(),
		Kind:           kind,
		ModuleId:       moduleID,
		Version:        req.GetVersion(),
		ValuesTemplate: req.GetValuesTemplate(),
		DependsOn:      pq.StringArray(dependsOn),
		Outputs:        outputs,
		CreatedBy:      claims.Subject,
	}

	c, err = a.store.Create(ctx, c)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create component: %v", err)
	}

	return &componentv1.CreateComponentResponse{Component: c.ToProto()}, nil
}

func (a *api) GetComponent(ctx context.Context, req *componentv1.GetComponentRequest) (*componentv1.GetComponentResponse, error) {
	id, err := uuid.Parse(req.GetComponentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component ID: %v", err)
	}

	c, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "component not found: %s", id)
	}

	return &componentv1.GetComponentResponse{Component: c.ToProto()}, nil
}

func (a *api) ListComponents(ctx context.Context, req *componentv1.ListComponentsRequest) (*componentv1.ListComponentsResponse, error) {
	appID, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application_id: %v", err)
	}

	var envID uuid.UUID
	if envStr := req.GetEnvironmentId(); envStr != "" {
		envID, err = uuid.Parse(envStr)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid environment_id: %v", err)
		}
	}

	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	cs, err := a.store.ListByApplicationID(ctx, appID, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list components: %v", err)
	}

	var overlay map[uuid.UUID]*model.ComponentOverride
	if envID != uuid.Nil {
		overrides, err := a.overrides.ListByApplicationEnv(ctx, appID, envID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to load overrides: %v", err)
		}
		overlay = make(map[uuid.UUID]*model.ComponentOverride, len(overrides))
		for i := range overrides {
			overlay[overrides[i].ComponentId] = &overrides[i]
		}
	}

	resp := &componentv1.ListComponentsResponse{}
	for i := range cs {
		proto := cs[i].ToProto()
		if o, ok := overlay[cs[i].Id]; ok {
			o.ApplyTo(proto)
			proto.HasOverride = true
		}
		resp.Components = append(resp.Components, proto)
	}

	if len(cs) > 0 && int32(len(cs)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := cs[len(cs)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateComponent(ctx context.Context, req *componentv1.UpdateComponentRequest) (*componentv1.UpdateComponentResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	proto := req.GetComponent()
	if proto == nil {
		return nil, status.Error(codes.InvalidArgument, "component is required")
	}

	id, err := uuid.Parse(proto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component ID: %v", err)
	}

	existing, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "component not found: %s", id)
	}

	mask := req.GetUpdateMask()
	if mask == nil || len(mask.GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required; specify which fields to update")
	}

	fields := map[string]any{
		"updated_at": time.Now(),
	}

	for _, path := range mask.GetPaths() {
		switch path {
		case "name":
			fields["name"] = proto.GetName()
		case "description":
			fields["description"] = proto.GetDescription()
		case "module_id":
			modID, err := uuid.Parse(proto.GetModuleId())
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid module ID: %v", err)
			}
			mod, err := a.modStore.Get(ctx, modID)
			if err != nil {
				return nil, status.Errorf(codes.NotFound, "module not found: %s", modID)
			}
			k := model.DeriveComponentKind(mod.Type)
			if k == "" {
				return nil, status.Errorf(codes.Internal, "cannot derive kind from module type: %s", mod.Type)
			}
			fields["module_id"] = modID
			fields["kind"] = k
		case "version":
			fields["version"] = proto.GetVersion()
		case "values_template":
			if vt := proto.GetValuesTemplate(); vt != "" {
				if err := model.ValidateValuesTemplate(vt); err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid values_template: %v", err)
				}
			}
			fields["values_template"] = proto.GetValuesTemplate()
		case "depends_on":
			deps, err := model.ParseDependsOn(proto.GetDependsOn())
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid depends_on: %v", err)
			}
			if err := a.validateDependsOnApp(ctx, deps, existing.ApplicationId); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "%v", err)
			}
			fields["depends_on"] = pq.StringArray(deps)
		case "outputs":
			outputs := model.ComponentOutputsFromProto(proto.GetOutputs())
			if existing.Kind == model.ComponentKindInfrastructure {
				outputs = nil
			}
			fields["outputs"] = outputs
		case "application_id", "kind", "id", "created_by", "created_at", "updated_at":
			return nil, status.Errorf(codes.InvalidArgument, "field %s is immutable", path)
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
		}
	}

	c, err := a.store.Update(ctx, existing, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update component: %v", err)
	}

	return &componentv1.UpdateComponentResponse{Component: c.ToProto()}, nil
}

func (a *api) DeleteComponent(ctx context.Context, req *componentv1.DeleteComponentRequest) (*componentv1.DeleteComponentResponse, error) {
	id, err := uuid.Parse(req.GetComponentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component ID: %v", err)
	}

	existing, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "component not found: %s", id)
	}

	// Check that no other components in the same app depend on this one.
	peers, err := a.store.ListByApplicationID(ctx, existing.ApplicationId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check dependents: %v", err)
	}
	idStr := id.String()
	for _, p := range peers {
		if p.Id == id {
			continue
		}
		for _, dep := range p.DependsOn {
			if dep == idStr {
				return nil, status.Errorf(codes.FailedPrecondition,
					"component %s is depended on by %s; remove the dependency first",
					existing.Name, p.Name)
			}
		}
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "component not found: %s", id)
	}

	return &componentv1.DeleteComponentResponse{}, nil
}

func (a *api) SetComponentOverride(ctx context.Context, req *componentv1.SetComponentOverrideRequest) (*componentv1.SetComponentOverrideResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	componentID, err := uuid.Parse(req.GetComponentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component ID: %v", err)
	}
	comp, err := a.store.Get(ctx, componentID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "component not found: %s", componentID)
	}

	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}
	env, err := a.envStore.Get(ctx, envID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", envID)
	}
	if env.ApplicationId != comp.ApplicationId {
		return nil, status.Errorf(codes.InvalidArgument,
			"environment %s does not belong to component's application %s", envID, comp.ApplicationId)
	}

	o := &model.ComponentOverride{
		ComponentId:   componentID,
		EnvironmentId: envID,
		Disabled:      req.GetDisabled(),
		CreatedBy:     claims.Subject,
	}

	if req.ModuleId != nil {
		modID, err := uuid.Parse(req.GetModuleId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid override module ID: %v", err)
		}
		if _, err := a.modStore.Get(ctx, modID); err != nil {
			return nil, status.Errorf(codes.NotFound, "override module not found: %s", modID)
		}
		o.ModuleId = &modID
	}
	if req.Version != nil {
		v := req.GetVersion()
		o.Version = &v
	}
	if req.ValuesTemplate != nil {
		vt := req.GetValuesTemplate()
		if vt != "" {
			if err := model.ValidateValuesTemplate(vt); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid override values_template: %v", err)
			}
		}
		o.ValuesTemplate = &vt
	}
	if deps := req.GetDependsOn(); deps != nil {
		parsed, err := model.ParseDependsOn(deps)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid depends_on: %v", err)
		}
		if err := a.validateDependsOnApp(ctx, parsed, comp.ApplicationId); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		o.DependsOn = pq.StringArray(parsed)
	}
	if outs := req.GetOutputs(); outs != nil {
		co := model.ComponentOutputsFromProto(outs)
		o.Outputs = &co
	}

	saved, err := a.overrides.Set(ctx, o)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set override: %v", err)
	}

	return &componentv1.SetComponentOverrideResponse{Override: saved.ToProto()}, nil
}

func (a *api) GetComponentOverride(ctx context.Context, req *componentv1.GetComponentOverrideRequest) (*componentv1.GetComponentOverrideResponse, error) {
	componentID, err := uuid.Parse(req.GetComponentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component ID: %v", err)
	}
	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	o, err := a.overrides.Get(ctx, componentID, envID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "component override not found")
	}

	return &componentv1.GetComponentOverrideResponse{Override: o.ToProto()}, nil
}

func (a *api) ListComponentOverrides(ctx context.Context, req *componentv1.ListComponentOverridesRequest) (*componentv1.ListComponentOverridesResponse, error) {
	componentID, err := uuid.Parse(req.GetComponentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component ID: %v", err)
	}

	os, err := a.overrides.ListByComponentID(ctx, componentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list overrides: %v", err)
	}

	resp := &componentv1.ListComponentOverridesResponse{}
	for i := range os {
		resp.Overrides = append(resp.Overrides, os[i].ToProto())
	}
	return resp, nil
}

func (a *api) DeleteComponentOverride(ctx context.Context, req *componentv1.DeleteComponentOverrideRequest) (*componentv1.DeleteComponentOverrideResponse, error) {
	componentID, err := uuid.Parse(req.GetComponentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component ID: %v", err)
	}
	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment ID: %v", err)
	}

	if err := a.overrides.Delete(ctx, componentID, envID); err != nil {
		return nil, status.Errorf(codes.NotFound, "component override not found")
	}

	return &componentv1.DeleteComponentOverrideResponse{}, nil
}

func (a *api) validateDependsOnApp(ctx context.Context, deps []string, appID uuid.UUID) error {
	for _, d := range deps {
		id, err := uuid.Parse(d)
		if err != nil {
			return fmt.Errorf("invalid dependency ID: %s", d)
		}
		dep, err := a.store.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("dependency component not found: %s", id)
		}
		if dep.ApplicationId != appID {
			return fmt.Errorf("dependency %s belongs to a different application", id)
		}
	}
	return nil
}
