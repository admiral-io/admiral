package source

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.admiral.io/admiral/internal/backend"
	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
	sourcev1 "go.admiral.io/sdk/proto/admiral/source/v1"
)

const Name = "endpoint.source"

var filterColumns = []string{"name", "type", "catalog"}

type api struct {
	store     *store.SourceStore
	credStore *store.CredentialStore
	qb        querybuilder.QueryBuilder
	logger    *zap.Logger
	scope     tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	srcStore, err := store.NewSourceStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	credStore, err := store.NewCredentialStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	return &api{
		store:     srcStore,
		credStore: credStore,
		logger:    log.Named(Name),
		scope:     scope.SubScope("source"),
		qb:        querybuilder.New(filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	sourcev1.RegisterSourceAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(sourcev1.RegisterSourceAPIHandler)
}

func (a *api) CreateSource(ctx context.Context, req *sourcev1.CreateSourceRequest) (*sourcev1.CreateSourceResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	src := &model.Source{
		Name:         req.GetName(),
		Description:  req.GetDescription(),
		Type:         model.SourceTypeFromProto(req.GetType()),
		URL:          req.GetUrl(),
		Catalog:      req.GetCatalog(),
		SourceConfig: model.SourceConfigFromCreateRequest(req),
		Labels:       model.Labels(req.GetLabels()),
		CreatedBy:    claims.Subject,
	}

	if req.CredentialId != nil {
		credID, err := uuid.Parse(req.GetCredentialId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid credential ID: %v", err)
		}
		src.CredentialId = &credID
	}

	src, err = a.store.Create(ctx, src)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSourceConfig) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to create source: %v", err)
	}

	return &sourcev1.CreateSourceResponse{
		Source: src.ToProto(),
	}, nil
}

func (a *api) GetSource(ctx context.Context, req *sourcev1.GetSourceRequest) (*sourcev1.GetSourceResponse, error) {
	id, err := uuid.Parse(req.GetSourceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid source ID: %v", err)
	}

	src, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "source not found: %s", id)
	}

	return &sourcev1.GetSourceResponse{
		Source: src.ToProto(),
	}, nil
}

func (a *api) ListSources(ctx context.Context, req *sourcev1.ListSourcesRequest) (*sourcev1.ListSourcesResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	srcs, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list sources: %v", err)
	}

	resp := &sourcev1.ListSourcesResponse{}
	for _, src := range srcs {
		resp.Sources = append(resp.Sources, src.ToProto())
	}

	if len(srcs) > 0 && int32(len(srcs)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := srcs[len(srcs)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateSource(ctx context.Context, req *sourcev1.UpdateSourceRequest) (*sourcev1.UpdateSourceResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	srcProto := req.GetSource()
	if srcProto == nil {
		return nil, status.Error(codes.InvalidArgument, "source is required")
	}

	id, err := uuid.Parse(srcProto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid source ID: %v", err)
	}

	src, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "source not found: %s", id)
	}

	fields := map[string]any{
		"updated_at": time.Now(),
	}

	mask := req.GetUpdateMask()
	if mask == nil || len(mask.GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required; specify which fields to update")
	}

	for _, path := range mask.GetPaths() {
		switch path {
		case "name":
			fields["name"] = srcProto.GetName()
		case "description":
			fields["description"] = srcProto.GetDescription()
		case "url":
			fields["url"] = srcProto.GetUrl()
		case "catalog":
			fields["catalog"] = srcProto.GetCatalog()
		case "source_config":
			fields["source_config"] = model.SourceConfigFromProto(srcProto)
		case "labels":
			fields["labels"] = model.Labels(srcProto.GetLabels())
		case "credential_id":
			if srcProto.CredentialId == nil {
				fields["credential_id"] = nil
				break
			}
			credID, err := uuid.Parse(srcProto.GetCredentialId())
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid credential ID: %v", err)
			}
			fields["credential_id"] = credID
		case "type":
			return nil, status.Error(codes.InvalidArgument, "source type is immutable; delete and recreate to change types")
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
		}
	}

	src, err = a.store.Update(ctx, src, fields)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSourceConfig) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to update source: %v", err)
	}

	return &sourcev1.UpdateSourceResponse{
		Source: src.ToProto(),
	}, nil
}

func (a *api) DeleteSource(ctx context.Context, req *sourcev1.DeleteSourceRequest) (*sourcev1.DeleteSourceResponse, error) {
	id, err := uuid.Parse(req.GetSourceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid source ID: %v", err)
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "source not found: %s", id)
	}

	return &sourcev1.DeleteSourceResponse{}, nil
}

func (a *api) TestSource(ctx context.Context, req *sourcev1.TestSourceRequest) (*sourcev1.TestSourceResponse, error) {
	id, err := uuid.Parse(req.GetSourceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid source ID: %v", err)
	}

	src, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "source not found: %s", id)
	}

	d, err := backend.For(src.Type)
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

	testErr := d.Probe(ctx, cred, src)

	statusStr := model.SourceTestStatusSuccess
	errMsg := ""
	if testErr != nil {
		statusStr = model.SourceTestStatusFailure
		errMsg = testErr.Error()
	}
	now := time.Now()

	updated, err := a.store.Update(ctx, src, map[string]any{
		"last_test_status": statusStr,
		"last_test_error":  errMsg,
		"last_tested_at":   now,
		"updated_at":       now,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record test outcome: %v", err)
	}

	resp := &sourcev1.TestSourceResponse{
		Error:  errMsg,
		Source: updated.ToProto(),
	}
	if testErr != nil {
		resp.Status = sourcev1.SourceTestStatus_SOURCE_TEST_STATUS_FAILURE
	} else {
		resp.Status = sourcev1.SourceTestStatus_SOURCE_TEST_STATUS_SUCCESS
	}
	return resp, nil
}

func (a *api) ListSourceVersions(ctx context.Context, req *sourcev1.ListSourceVersionsRequest) (*sourcev1.ListSourceVersionsResponse, error) {
	id, err := uuid.Parse(req.GetSourceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid source ID: %v", err)
	}

	src, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "source not found: %s", id)
	}

	d, err := backend.For(src.Type)
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

	versions, err := d.ListVersions(ctx, cred, src)
	if err != nil {
		if errors.Is(err, backend.ErrOperationNotSupported) {
			return nil, status.Errorf(codes.Unimplemented, "ListSourceVersions not yet supported for source type %s", src.Type)
		}
		return nil, status.Errorf(codes.Internal, "list versions: %v", err)
	}

	resp := &sourcev1.ListSourceVersionsResponse{}
	for _, v := range versions {
		sv := &sourcev1.SourceVersion{
			Version:     v.Name,
			Description: v.Description,
		}
		if v.PublishedAt != nil {
			sv.PublishedAt = timestamppb.New(*v.PublishedAt)
		}
		resp.Versions = append(resp.Versions, sv)
	}
	return resp, nil
}
