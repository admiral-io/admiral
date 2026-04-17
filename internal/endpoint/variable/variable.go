package variable

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
	variablev1 "go.admiral.io/sdk/proto/admiral/variable/v1"
)

const Name = "endpoint.variable"

var filterColumns = []string{"key", "sensitive", "type", "source", "application_id", "environment_id"}

type api struct {
	store    *store.VariableStore
	appStore *store.ApplicationStore
	envStore *store.EnvironmentStore
	qb       querybuilder.QueryBuilder
	logger   *zap.Logger
	scope    tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	varStore, err := store.NewVariableStore(db.GormDB())
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
		store:    varStore,
		appStore: appStore,
		envStore: envStore,
		logger:   log.Named(Name),
		scope:    scope.SubScope("variable"),
		qb:       querybuilder.New(filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	variablev1.RegisterVariableAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(variablev1.RegisterVariableAPIHandler)
}

func (a *api) CreateVariable(ctx context.Context, req *variablev1.CreateVariableRequest) (*variablev1.CreateVariableResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	var appID *uuid.UUID
	if req.ApplicationId != nil {
		id, err := uuid.Parse(*req.ApplicationId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid application_id: %v", err)
		}
		if _, err := a.appStore.Get(ctx, id); err != nil {
			return nil, status.Errorf(codes.NotFound, "application not found: %s", id)
		}
		appID = &id
	}

	var envID *uuid.UUID
	if req.EnvironmentId != nil {
		if appID == nil {
			return nil, status.Error(codes.InvalidArgument, "environment_id requires application_id")
		}
		id, err := uuid.Parse(*req.EnvironmentId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid environment_id: %v", err)
		}
		env, err := a.envStore.Get(ctx, id)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "environment not found: %s", id)
		}
		if env.ApplicationId != *appID {
			return nil, status.Error(codes.InvalidArgument, "environment does not belong to the specified application")
		}
		envID = &id
	}

	exists, err := a.store.ExistsByKey(ctx, req.GetKey(), appID, envID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check key uniqueness: %v", err)
	}
	if exists {
		return nil, status.Errorf(codes.AlreadyExists, "variable with key %q already exists at this scope", req.GetKey())
	}

	varType := model.VariableTypeString
	if req.GetType() != variablev1.VariableType_VARIABLE_TYPE_UNSPECIFIED {
		varType = model.VariableTypeFromProto(req.GetType())
		if varType == "" {
			return nil, status.Errorf(codes.InvalidArgument, "unsupported variable type: %v", req.GetType())
		}
	}

	v := &model.Variable{
		Key:           req.GetKey(),
		Value:         req.GetValue(),
		Sensitive:     req.GetSensitive(),
		Type:          varType,
		Source:        model.VariableSourceUser,
		Description:   req.GetDescription(),
		ApplicationId: appID,
		EnvironmentId: envID,
		CreatedBy:     claims.Subject,
	}

	v, err = a.store.Create(ctx, v)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create variable: %v", err)
	}

	return &variablev1.CreateVariableResponse{Variable: v.ToProto()}, nil
}

func (a *api) GetVariable(ctx context.Context, req *variablev1.GetVariableRequest) (*variablev1.GetVariableResponse, error) {
	id, err := uuid.Parse(req.GetVariableId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid variable ID: %v", err)
	}

	v, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "variable not found: %s", id)
	}

	return &variablev1.GetVariableResponse{Variable: v.ToProto()}, nil
}

func (a *api) ListVariables(ctx context.Context, req *variablev1.ListVariablesRequest) (*variablev1.ListVariablesResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	vs, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list variables: %v", err)
	}

	resp := &variablev1.ListVariablesResponse{}
	for i := range vs {
		resp.Variables = append(resp.Variables, vs[i].ToProto())
	}

	if len(vs) > 0 && int32(len(vs)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := vs[len(vs)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateVariable(ctx context.Context, req *variablev1.UpdateVariableRequest) (*variablev1.UpdateVariableResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	proto := req.GetVariable()
	if proto == nil {
		return nil, status.Error(codes.InvalidArgument, "variable is required")
	}

	id, err := uuid.Parse(proto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid variable ID: %v", err)
	}

	existing, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "variable not found: %s", id)
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
		case "value":
			fields["value"] = proto.GetValue()
		case "sensitive":
			fields["sensitive"] = proto.GetSensitive()
		case "type":
			varType := model.VariableTypeFromProto(proto.GetType())
			if varType == "" {
				return nil, status.Errorf(codes.InvalidArgument, "unsupported variable type: %v", proto.GetType())
			}
			fields["type"] = varType
		case "description":
			fields["description"] = proto.GetDescription()
		case "key", "application_id", "environment_id", "source", "id", "created_by", "created_at", "updated_at":
			return nil, status.Errorf(codes.InvalidArgument, "field %s is immutable", path)
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
		}
	}

	// If value or type changed, re-validate.
	if _, hasValue := fields["value"]; hasValue {
		check := *existing
		check.Value = fields["value"].(string)
		if t, hasType := fields["type"]; hasType {
			check.Type = t.(string)
		}
		if err := check.Validate(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid variable: %v", err)
		}
	} else if _, hasType := fields["type"]; hasType {
		check := *existing
		check.Type = fields["type"].(string)
		if err := check.Validate(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid variable: %v", err)
		}
	}

	v, err := a.store.Update(ctx, existing, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update variable: %v", err)
	}

	return &variablev1.UpdateVariableResponse{Variable: v.ToProto()}, nil
}

func (a *api) DeleteVariable(ctx context.Context, req *variablev1.DeleteVariableRequest) (*variablev1.DeleteVariableResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	id, err := uuid.Parse(req.GetVariableId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid variable ID: %v", err)
	}

	existing, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "variable not found: %s", id)
	}

	if existing.Source == model.VariableSourceInfrastructure {
		return nil, status.Error(codes.FailedPrecondition, "infrastructure variables are system-managed and cannot be deleted via API")
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete variable: %v", err)
	}

	return &variablev1.DeleteVariableResponse{}, nil
}
