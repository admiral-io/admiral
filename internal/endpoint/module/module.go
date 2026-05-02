package module

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

	"go.admiral.io/admiral/internal/backend"
	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/service/encryption"
	"go.admiral.io/admiral/internal/store"
	modulev1 "go.admiral.io/sdk/proto/admiral/module/v1"
)

const Name = "endpoint.module"

var filterColumns = []string{"name", "type", "source_id"}

type api struct {
	store     *store.ModuleStore
	srcStore  *store.SourceStore
	credStore *store.CredentialStore
	compStore *store.ComponentStore
	revStore  *store.RevisionStore
	qb        querybuilder.QueryBuilder
	logger    *zap.Logger
	scope     tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	enc, err := service.GetService[encryption.Service](encryption.Name)
	if err != nil {
		return nil, err
	}

	db, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}

	modStore, err := store.NewModuleStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	srcStore, err := store.NewSourceStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	credStore, err := store.NewCredentialStore(db.GormDB(), enc)
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

	return &api{
		store:     modStore,
		srcStore:  srcStore,
		credStore: credStore,
		compStore: compStore,
		revStore:  revStore,
		logger:    log.Named(Name),
		scope:     scope.SubScope("module"),
		qb:        querybuilder.New("modules", filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	modulev1.RegisterModuleAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(modulev1.RegisterModuleAPIHandler)
}

func (a *api) CreateModule(ctx context.Context, req *modulev1.CreateModuleRequest) (*modulev1.CreateModuleResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	sourceID, err := uuid.Parse(req.GetSourceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid source ID: %v", err)
	}

	src, err := a.srcStore.Get(ctx, sourceID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "source not found: %s", sourceID)
	}

	modType := model.ModuleTypeFromProto(req.GetType())
	if err := model.ValidateModuleSourceCompat(modType, src.Type); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	mod := &model.Module{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Type:        modType,
		SourceId:    sourceID,
		Ref:         req.GetRef(),
		Root:        req.GetRoot(),
		Path:        req.GetPath(),
		Labels:      model.Labels(req.GetLabels()),
		CreatedBy:   claims.Subject,
	}

	mod, err = a.store.Create(ctx, mod)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create module: %v", err)
	}

	return &modulev1.CreateModuleResponse{
		Module: mod.ToProto(),
	}, nil
}

func (a *api) GetModule(ctx context.Context, req *modulev1.GetModuleRequest) (*modulev1.GetModuleResponse, error) {
	id, err := uuid.Parse(req.GetModuleId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid module ID: %v", err)
	}

	mod, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "module not found: %s", id)
	}

	return &modulev1.GetModuleResponse{
		Module: mod.ToProto(),
	}, nil
}

func (a *api) ListModules(ctx context.Context, req *modulev1.ListModulesRequest) (*modulev1.ListModulesResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	mods, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list modules: %v", err)
	}

	resp := &modulev1.ListModulesResponse{}
	for _, mod := range mods {
		resp.Modules = append(resp.Modules, mod.ToProto())
	}

	if len(mods) > 0 && int32(len(mods)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := mods[len(mods)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateModule(ctx context.Context, req *modulev1.UpdateModuleRequest) (*modulev1.UpdateModuleResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	modProto := req.GetModule()
	if modProto == nil {
		return nil, status.Error(codes.InvalidArgument, "module is required")
	}

	id, err := uuid.Parse(modProto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid module ID: %v", err)
	}

	mod, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "module not found: %s", id)
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
			fields["name"] = modProto.GetName()
		case "description":
			fields["description"] = modProto.GetDescription()
		case "source_id":
			sourceID, err := uuid.Parse(modProto.GetSourceId())
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid source ID: %v", err)
			}
			src, err := a.srcStore.Get(ctx, sourceID)
			if err != nil {
				return nil, status.Errorf(codes.NotFound, "source not found: %s", sourceID)
			}
			if err := model.ValidateModuleSourceCompat(mod.Type, src.Type); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "%v", err)
			}
			fields["source_id"] = sourceID
		case "ref":
			fields["ref"] = modProto.GetRef()
		case "root":
			fields["root"] = modProto.GetRoot()
		case "path":
			fields["path"] = modProto.GetPath()
		case "labels":
			fields["labels"] = model.Labels(modProto.GetLabels())
		case "type":
			return nil, status.Error(codes.InvalidArgument, "module type is immutable; delete and recreate to change types")
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
		}
	}

	mod, err = a.store.Update(ctx, mod, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update module: %v", err)
	}

	return &modulev1.UpdateModuleResponse{
		Module: mod.ToProto(),
	}, nil
}

func (a *api) DeleteModule(ctx context.Context, req *modulev1.DeleteModuleRequest) (*modulev1.DeleteModuleResponse, error) {
	id, err := uuid.Parse(req.GetModuleId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid module ID: %v", err)
	}

	refCount, err := a.compStore.CountByModuleID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check module references: %v", err)
	}
	if refCount > 0 {
		return nil, status.Errorf(codes.FailedPrecondition,
			"module is in use by %d component(s); delete or reassign them before deleting this module", refCount)
	}

	revCount, err := a.revStore.CountByModuleID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check module references: %v", err)
	}
	if revCount > 0 {
		return nil, status.Errorf(codes.FailedPrecondition,
			"module is referenced by %d historical revision(s); module versions cannot be removed once they have been deployed", revCount)
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "module not found: %s", id)
	}

	return &modulev1.DeleteModuleResponse{}, nil
}

func (a *api) ResolveModule(ctx context.Context, req *modulev1.ResolveModuleRequest) (*modulev1.ResolveModuleResponse, error) {
	id, err := uuid.Parse(req.GetModuleId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid module ID: %v", err)
	}

	mod, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "module not found: %s", id)
	}

	src, err := a.srcStore.Get(ctx, mod.SourceId)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "source not found: %s", mod.SourceId)
	}

	b, err := backend.For(src.Type)
	if err != nil {
		return nil, status.Errorf(codes.Unimplemented, "no backend registered for source type %s", src.Type)
	}

	var cred *model.Credential
	if src.CredentialId != nil {
		cred, err = a.credStore.Get(ctx, *src.CredentialId)
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "attached credential not found: %v", err)
		}
	}

	ref := mod.Ref
	if req.GetRefOverride() != "" {
		ref = req.GetRefOverride()
	}

	res, err := b.Fetch(ctx, cred, src, backend.FetchOptions{
		Ref:  ref,
		Root: mod.Root,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resolve module: %v", err)
	}
	defer res.Cleanup()

	return &modulev1.ResolveModuleResponse{
		Revision: res.Revision,
		Digest:   res.Digest,
		Module:   mod.ToProto(),
	}, nil
}
