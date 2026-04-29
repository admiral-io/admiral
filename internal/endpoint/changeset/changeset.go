package changeset

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
	"go.admiral.io/admiral/internal/service/orchestration"
	"go.admiral.io/admiral/internal/store"
	changesetv1 "go.admiral.io/sdk/proto/admiral/changeset/v1"
	variablev1 "go.admiral.io/sdk/proto/admiral/variable/v1"
)

const Name = "endpoint.changeset"

var filterColumns = []string{"application_id", "environment_id", "status"}

type api struct {
	changesetv1.UnimplementedChangeSetAPIServer

	store     *store.ChangeSetStore
	compStore *store.ComponentStore
	modStore  *store.ModuleStore
	appStore  *store.ApplicationStore
	envStore  *store.EnvironmentStore
	revStore  *store.RevisionStore
	runStore  *store.RunStore
	orch      *orchestration.Service
	qb        querybuilder.QueryBuilder
	logger    *zap.Logger
	scope     tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}

	csStore, err := store.NewChangeSetStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	compStore, err := store.NewComponentStore(db.GormDB())
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
	revStore, err := store.NewRevisionStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	runStore, err := store.NewRunStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	orch, err := service.GetService[*orchestration.Service](orchestration.Name)
	if err != nil {
		return nil, err
	}

	return &api{
		store:     csStore,
		compStore: compStore,
		modStore:  modStore,
		appStore:  appStore,
		envStore:  envStore,
		revStore:  revStore,
		runStore:  runStore,
		orch:      orch,
		logger:    log.Named(Name),
		scope:     scope.SubScope("changeset"),
		qb:        querybuilder.New("change_sets", filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	changesetv1.RegisterChangeSetAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(changesetv1.RegisterChangeSetAPIHandler)
}

func (a *api) CreateChangeSet(ctx context.Context, req *changesetv1.CreateChangeSetRequest) (*changesetv1.CreateChangeSetResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
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
	if env.ApplicationId != app.Id {
		return nil, status.Errorf(codes.InvalidArgument,
			"environment %s does not belong to application %s", envID, app.Id)
	}

	base, err := a.revStore.LatestSucceededByEnv(ctx, appID, envID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to snapshot env head: %v", err)
	}

	cs := &model.ChangeSet{
		ApplicationId:     appID,
		EnvironmentId:     envID,
		Status:            model.ChangeSetStatusOpen,
		Title:             req.GetTitle(),
		Description:       req.GetDescription(),
		BaseHeadRevisions: base,
		CreatedBy:         claims.Subject,
	}

	cs, err = a.store.Create(ctx, cs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create change set: %v", err)
	}

	return &changesetv1.CreateChangeSetResponse{ChangeSet: cs.ToProto(nil, nil)}, nil
}

func (a *api) GetChangeSet(ctx context.Context, req *changesetv1.GetChangeSetRequest) (*changesetv1.GetChangeSetResponse, error) {
	cs, err := a.store.GetByIdentifier(ctx, req.GetChangeSetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	entries, err := a.store.ListEntries(ctx, cs.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list entries: %v", err)
	}
	vars, err := a.store.ListVariableEntries(ctx, cs.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list variable entries: %v", err)
	}
	if entries == nil {
		entries = []model.ChangeSetEntry{}
	}
	if vars == nil {
		vars = []model.ChangeSetVariableEntry{}
	}

	return &changesetv1.GetChangeSetResponse{ChangeSet: cs.ToProto(entries, vars)}, nil
}

func (a *api) ListChangeSets(ctx context.Context, req *changesetv1.ListChangeSetsRequest) (*changesetv1.ListChangeSetsResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	sets, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list change sets: %v", err)
	}

	resp := &changesetv1.ListChangeSetsResponse{}
	for i := range sets {
		resp.ChangeSets = append(resp.ChangeSets, sets[i].ToProto(nil, nil))
	}

	if len(sets) > 0 && int32(len(sets)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := sets[len(sets)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateChangeSet(ctx context.Context, req *changesetv1.UpdateChangeSetRequest) (*changesetv1.UpdateChangeSetResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	proto := req.GetChangeSet()
	if proto == nil {
		return nil, status.Error(codes.InvalidArgument, "change_set is required")
	}

	id, err := uuid.Parse(proto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid change_set id: %v", err)
	}

	existing, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "change set not found: %s", id)
	}
	if err := existing.RequireMutable(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	mask := req.GetUpdateMask()
	if mask == nil || len(mask.GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required")
	}

	fields := map[string]any{
		"updated_at": time.Now(),
	}
	for _, path := range mask.GetPaths() {
		switch path {
		case "title":
			fields["title"] = proto.GetTitle()
		case "description":
			fields["description"] = proto.GetDescription()
		case "id", "application_id", "environment_id", "status", "copied_from_id",
			"run_id", "entries", "variable_entries",
			"created_by", "created_at", "updated_at":
			return nil, status.Errorf(codes.InvalidArgument, "field %s is immutable", path)
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
		}
	}

	cs, err := a.store.Update(ctx, existing, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update change set: %v", err)
	}

	return &changesetv1.UpdateChangeSetResponse{ChangeSet: cs.ToProto(nil, nil)}, nil
}

func (a *api) DiscardChangeSet(ctx context.Context, req *changesetv1.DiscardChangeSetRequest) (*changesetv1.DiscardChangeSetResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	existing, err := a.store.GetByIdentifier(ctx, req.GetChangeSetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	if err := existing.RequireMutable(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	cs, err := a.store.Update(ctx, existing, map[string]any{
		"status":     model.ChangeSetStatusDiscarded,
		"updated_at": time.Now(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to discard change set: %v", err)
	}

	return &changesetv1.DiscardChangeSetResponse{ChangeSet: cs.ToProto(nil, nil)}, nil
}

func (a *api) CopyChangeSet(ctx context.Context, req *changesetv1.CopyChangeSetRequest) (*changesetv1.CopyChangeSetResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	src, err := a.store.GetByIdentifier(ctx, req.GetChangeSetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment_id: %v", err)
	}
	env, err := a.envStore.Get(ctx, envID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "environment not found: %s", envID)
	}
	if env.ApplicationId != src.ApplicationId {
		return nil, status.Errorf(codes.InvalidArgument,
			"environment %s does not belong to application %s", envID, src.ApplicationId)
	}

	base, err := a.revStore.LatestSucceededByEnv(ctx, src.ApplicationId, envID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to snapshot env head: %v", err)
	}

	cs, err := a.store.Copy(ctx, src, envID, req.GetTitle(), req.GetDescription(), claims.Subject, base)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to copy change set: %v", err)
	}

	entries, err := a.store.ListEntries(ctx, cs.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list entries: %v", err)
	}
	vars, err := a.store.ListVariableEntries(ctx, cs.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list variable entries: %v", err)
	}

	return &changesetv1.CopyChangeSetResponse{ChangeSet: cs.ToProto(entries, vars)}, nil
}

func (a *api) SetEntry(ctx context.Context, req *changesetv1.SetEntryRequest) (*changesetv1.SetEntryResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	cs, err := a.openChangeSet(ctx, req.GetChangeSetId())
	if err != nil {
		return nil, err
	}
	if err := a.supersedeIfActive(ctx, cs); err != nil {
		return nil, err
	}

	if err := model.ValidateSlug(req.GetComponentSlug()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component_slug: %v", err)
	}
	if err := model.ValidateChangeSetEntryType(req.GetChangeType()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	entry := &model.ChangeSetEntry{
		ChangeSetId:   cs.Id,
		ComponentSlug: req.GetComponentSlug(),
		ChangeType:    req.GetChangeType(),
		DependsOn:     pq.StringArray(req.GetDependsOn()),
	}

	if req.ModuleId != nil {
		modID, err := uuid.Parse(req.GetModuleId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid module_id: %v", err)
		}
		if _, err := a.modStore.Get(ctx, modID); err != nil {
			return nil, status.Errorf(codes.NotFound, "module not found: %s", modID)
		}
		entry.ModuleId = &modID
	}
	if req.Version != nil {
		v := req.GetVersion()
		entry.Version = &v
	}
	if req.ValuesTemplate != nil {
		v := req.GetValuesTemplate()
		entry.ValuesTemplate = &v
	}
	if req.Description != nil {
		v := req.GetDescription()
		entry.Description = &v
	}

	switch req.GetChangeType() {
	case model.ChangeSetEntryTypeCreate:
		// Reject CREATE if a component with this slug already exists in the
		// application -- the operator wanted UPDATE.
		if existing, err := a.compStore.GetByApplicationSlug(ctx, cs.ApplicationId, req.GetComponentSlug()); err == nil && existing != nil {
			return nil, status.Errorf(codes.AlreadyExists,
				"component %s already exists in application %s; use UPDATE instead",
				req.GetComponentSlug(), cs.ApplicationId)
		}
	case model.ChangeSetEntryTypeUpdate, model.ChangeSetEntryTypeDestroy, model.ChangeSetEntryTypeOrphan:
		comp, err := a.compStore.GetByApplicationSlug(ctx, cs.ApplicationId, req.GetComponentSlug())
		if err != nil {
			return nil, status.Errorf(codes.NotFound,
				"component %s not found in application %s", req.GetComponentSlug(), cs.ApplicationId)
		}
		entry.ComponentId = &comp.Id
	}

	saved, err := a.store.UpsertEntry(ctx, entry)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &changesetv1.SetEntryResponse{Entry: saved.ToProto()}, nil
}

func (a *api) RemoveEntry(ctx context.Context, req *changesetv1.RemoveEntryRequest) (*changesetv1.RemoveEntryResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	cs, err := a.openChangeSet(ctx, req.GetChangeSetId())
	if err != nil {
		return nil, err
	}
	if err := a.supersedeIfActive(ctx, cs); err != nil {
		return nil, err
	}
	if err := model.ValidateSlug(req.GetComponentSlug()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid component_slug: %v", err)
	}

	if err := a.store.DeleteEntryBySlug(ctx, cs.Id, req.GetComponentSlug()); err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	return &changesetv1.RemoveEntryResponse{}, nil
}

func (a *api) SetVariable(ctx context.Context, req *changesetv1.SetVariableRequest) (*changesetv1.SetVariableResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	cs, err := a.openChangeSet(ctx, req.GetChangeSetId())
	if err != nil {
		return nil, err
	}
	if err := a.supersedeIfActive(ctx, cs); err != nil {
		return nil, err
	}

	varType := model.VariableTypeFromProto(req.GetType())
	if varType == "" {
		if req.GetType() != variablev1.VariableType_VARIABLE_TYPE_UNSPECIFIED {
			return nil, status.Errorf(codes.InvalidArgument, "invalid variable type")
		}
		varType = model.VariableTypeString
	}

	val := req.GetValue()
	entry := &model.ChangeSetVariableEntry{
		ChangeSetId: cs.Id,
		Key:         req.GetKey(),
		Value:       &val,
		Type:        varType,
		Sensitive:   req.GetSensitive(),
	}

	saved, err := a.store.UpsertVariableEntry(ctx, entry)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &changesetv1.SetVariableResponse{VariableEntry: saved.ToProto()}, nil
}

func (a *api) RemoveVariable(ctx context.Context, req *changesetv1.RemoveVariableRequest) (*changesetv1.RemoveVariableResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	cs, err := a.openChangeSet(ctx, req.GetChangeSetId())
	if err != nil {
		return nil, err
	}
	if err := a.supersedeIfActive(ctx, cs); err != nil {
		return nil, err
	}

	tombstone := &model.ChangeSetVariableEntry{
		ChangeSetId: cs.Id,
		Key:         req.GetKey(),
		Value:       nil,
		Type:        model.VariableTypeString,
	}
	saved, err := a.store.UpsertVariableEntry(ctx, tombstone)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &changesetv1.RemoveVariableResponse{VariableEntry: saved.ToProto()}, nil
}

// openChangeSet loads a change set by display ID or UUID and rejects when it
// is not OPEN. Used by every mutating RPC.
func (a *api) openChangeSet(ctx context.Context, ident string) (*model.ChangeSet, error) {
	cs, err := a.store.GetByIdentifier(ctx, ident)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	if err := cs.RequireMutable(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	return cs, nil
}

// supersedeIfActive marks any active run for the change set as SUPERSEDED,
// with two carve-outs:
//   - APPLYING: rejected (mid-flight; can't supersede). Operator must wait
//     for completion or cancel explicitly.
//   - terminal / no active run: no-op.
func (a *api) supersedeIfActive(ctx context.Context, cs *model.ChangeSet) error {
	run, err := a.runStore.FindActiveByChangeSet(ctx, cs.Id)
	if err != nil {
		return status.Errorf(codes.Internal, "check active run: %v", err)
	}
	if run == nil {
		return nil
	}

	if run.Status == model.RunStatusApplying {
		return status.Errorf(codes.FailedPrecondition,
			"change set has an active applying run %s; wait for it to complete or cancel it before editing",
			run.Id)
	}

	if err := a.orch.SupersedeRun(ctx, run); err != nil {
		return status.Errorf(codes.Internal, "%v", err)
	}
	a.logger.Info("change set edit superseded prior run",
		zap.String("change_set_id", cs.DisplayId),
		zap.String("run_id", run.Id.String()))

	return nil
}
