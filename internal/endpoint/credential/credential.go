package credential

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

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
	credentialv1 "go.admiral.io/sdk/proto/admiral/credential/v1"
)

const Name = "endpoint.credential"

var filterColumns = []string{"name", "type"}

type api struct {
	store       *store.CredentialStore
	sourceStore *store.SourceStore
	qb          querybuilder.QueryBuilder
	logger      *zap.Logger
	scope       tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	credStore, err := store.NewCredentialStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	srcStore, err := store.NewSourceStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	return &api{
		store:       credStore,
		sourceStore: srcStore,
		logger:      log.Named(Name),
		scope:       scope.SubScope("credential"),
		qb:          querybuilder.New(filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	credentialv1.RegisterCredentialAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(credentialv1.RegisterCredentialAPIHandler)
}

func (a *api) CreateCredential(ctx context.Context, req *credentialv1.CreateCredentialRequest) (*credentialv1.CreateCredentialResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	cred := &model.Credential{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Type:        model.CredentialTypeFromProto(req.GetType()),
		AuthConfig:  model.AuthConfigFromCreateRequest(req),
		Labels:      model.Labels(req.GetLabels()),
		CreatedBy:   claims.Subject,
	}

	cred, err = a.store.Create(ctx, cred)
	if err != nil {
		if errors.Is(err, store.ErrInvalidAuthConfig) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to create credential: %v", err)
	}

	return &credentialv1.CreateCredentialResponse{
		Credential: cred.ToProto(),
	}, nil
}

func (a *api) GetCredential(ctx context.Context, req *credentialv1.GetCredentialRequest) (*credentialv1.GetCredentialResponse, error) {
	id, err := uuid.Parse(req.GetCredentialId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid credential ID: %v", err)
	}

	cred, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "credential not found: %s", id)
	}

	return &credentialv1.GetCredentialResponse{
		Credential: cred.ToProto(),
	}, nil
}

func (a *api) ListCredentials(ctx context.Context, req *credentialv1.ListCredentialsRequest) (*credentialv1.ListCredentialsResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	creds, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list credentials: %v", err)
	}

	resp := &credentialv1.ListCredentialsResponse{}
	for _, cred := range creds {
		resp.Credentials = append(resp.Credentials, cred.ToProto())
	}

	if len(creds) > 0 && int32(len(creds)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := creds[len(creds)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateCredential(ctx context.Context, req *credentialv1.UpdateCredentialRequest) (*credentialv1.UpdateCredentialResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	credProto := req.GetCredential()
	if credProto == nil {
		return nil, status.Error(codes.InvalidArgument, "credential is required")
	}

	mask := req.GetUpdateMask()
	if mask == nil || len(mask.GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required; specify which fields to update")
	}

	id, err := uuid.Parse(credProto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid credential ID: %v", err)
	}

	cred, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "credential not found: %s", id)
	}

	fields := map[string]any{
		"updated_at": time.Now(),
	}

	for _, path := range mask.GetPaths() {
		switch path {
		case "name":
			fields["name"] = credProto.GetName()
		case "description":
			fields["description"] = credProto.GetDescription()
		case "labels":
			fields["labels"] = model.Labels(credProto.GetLabels())
		case "auth_config":
			fields["auth_config"] = model.AuthConfigFromProto(credProto)
		case "type":
			return nil, status.Error(codes.InvalidArgument, "credential type is immutable; delete and recreate to change types")
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
		}
	}

	cred, err = a.store.Update(ctx, cred, fields)
	if err != nil {
		if errors.Is(err, store.ErrInvalidAuthConfig) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to update credential: %v", err)
	}

	return &credentialv1.UpdateCredentialResponse{
		Credential: cred.ToProto(),
	}, nil
}

func (a *api) DeleteCredential(ctx context.Context, req *credentialv1.DeleteCredentialRequest) (*credentialv1.DeleteCredentialResponse, error) {
	id, err := uuid.Parse(req.GetCredentialId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid credential ID: %v", err)
	}
	
	refCount, err := a.sourceStore.CountByCredentialID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check credential references: %v", err)
	}
	if refCount > 0 {
		return nil, status.Errorf(codes.FailedPrecondition,
			"credential is in use by %d source(s); detach or delete them before deleting this credential", refCount)
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "credential not found: %s", id)
	}

	return &credentialv1.DeleteCredentialResponse{}, nil
}
