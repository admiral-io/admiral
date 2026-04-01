package application

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	commonv1 "buf.build/gen/go/admiral/common/protocolbuffers/go/admiral/common/v1"
	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
	applicationv1 "go.admiral.io/sdk/proto/admiral/api/application/v1"
)

const Name = "endpoint.application"

var filterColumns = []string{"name"}

type api struct {
	store  *store.ApplicationStore
	logger *zap.Logger
	scope  tally.Scope
	qb     querybuilder.QueryBuilder
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	appStore, err := store.NewApplicationStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	return &api{
		store:  appStore,
		logger: log.Named(Name),
		scope:  scope.SubScope("application"),
		qb:     querybuilder.New(filterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	applicationv1.RegisterApplicationAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(applicationv1.RegisterApplicationAPIHandler)
}

func (a *api) CreateApplication(ctx context.Context, req *applicationv1.CreateApplicationRequest) (*applicationv1.CreateApplicationResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	app := &model.Application{
		Name:        req.GetName(),
		Description: description,
		Labels:      model.Labels(req.GetLabels()),
		CreatedBy:   claims.Subject,
		UpdatedBy:   claims.Subject,
	}

	created, err := a.store.Create(ctx, app)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create application: %v", err)
	}

	return &applicationv1.CreateApplicationResponse{
		Application: applicationToProto(created),
	}, nil
}

func (a *api) GetApplication(ctx context.Context, req *applicationv1.GetApplicationRequest) (*applicationv1.GetApplicationResponse, error) {
	id, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application ID: %v", err)
	}

	app, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", id)
	}

	return &applicationv1.GetApplicationResponse{
		Application: applicationToProto(app),
	}, nil
}

func (a *api) ListApplications(ctx context.Context, req *applicationv1.ListApplicationsRequest) (*applicationv1.ListApplicationsResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	apps, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list applications: %v", err)
	}

	resp := &applicationv1.ListApplicationsResponse{}
	for _, app := range apps {
		resp.Applications = append(resp.Applications, applicationToProto(&app))
	}

	if len(apps) > 0 && int32(len(apps)) == effectiveLimit(req.GetPageSize()) {
		last := apps[len(apps)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateApplication(ctx context.Context, req *applicationv1.UpdateApplicationRequest) (*applicationv1.UpdateApplicationResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	appProto := req.GetApplication()
	if appProto == nil {
		return nil, status.Error(codes.InvalidArgument, "application is required")
	}

	id, err := uuid.Parse(appProto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application ID: %v", err)
	}

	existing, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", id)
	}

	fields := map[string]any{
		"updated_by": claims.Subject,
		"updated_at": time.Now(),
	}

	mask := req.GetUpdateMask()
	if mask == nil || len(mask.GetPaths()) == 0 {
		fields["name"] = appProto.GetName()
		fields["description"] = appProto.GetDescription()
		fields["labels"] = model.Labels(appProto.GetLabels())
	} else {
		for _, path := range mask.GetPaths() {
			switch path {
			case "name":
				fields["name"] = appProto.GetName()
			case "description":
				fields["description"] = appProto.GetDescription()
			case "labels":
				fields["labels"] = model.Labels(appProto.GetLabels())
			default:
				return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
			}
		}
	}

	updated, err := a.store.Update(ctx, existing, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update application: %v", err)
	}

	return &applicationv1.UpdateApplicationResponse{
		Application: applicationToProto(updated),
	}, nil
}

func (a *api) DeleteApplication(ctx context.Context, req *applicationv1.DeleteApplicationRequest) (*applicationv1.DeleteApplicationResponse, error) {
	id, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application ID: %v", err)
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", id)
	}

	return &applicationv1.DeleteApplicationResponse{}, nil
}

func applicationToProto(app *model.Application) *applicationv1.Application {
	return &applicationv1.Application{
		Id:          app.Id.String(),
		Name:        app.Name,
		Description: app.Description,
		Labels:      map[string]string(app.Labels),
		CreatedBy: &commonv1.ActorRef{
			Id: app.CreatedBy,
		},
		UpdatedBy: &commonv1.ActorRef{
			Id: app.UpdatedBy,
		},
		CreatedAt: timestamppb.New(app.CreatedAt),
		UpdatedAt: timestamppb.New(app.UpdatedAt),
	}
}

func effectiveLimit(pageSize int32) int32 {
	if pageSize <= 0 {
		return querybuilder.DefaultLimit
	}
	if pageSize > querybuilder.MaxResultLimit {
		return querybuilder.MaxResultLimit
	}
	return pageSize
}
