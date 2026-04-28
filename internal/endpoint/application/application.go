package application

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
	applicationv1 "go.admiral.io/sdk/proto/admiral/application/v1"
)

const Name = "endpoint.application"

var filterColumns = []string{"name"}

type api struct {
	store  *store.ApplicationStore
	qb     querybuilder.QueryBuilder
	logger *zap.Logger
	scope  tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service](database.Name)
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
		qb:     querybuilder.New("applications", filterColumns),
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
	}

	app, err = a.store.Create(ctx, app)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create application: %v", err)
	}

	return &applicationv1.CreateApplicationResponse{
		Application: app.ToProto(),
	}, nil
}

func (a *api) GetApplication(ctx context.Context, req *applicationv1.GetApplicationRequest) (*applicationv1.GetApplicationResponse, error) {
	id, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application id: %v", err)
	}

	app, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", id)
	}

	return &applicationv1.GetApplicationResponse{
		Application: app.ToProto(),
	}, nil
}

func (a *api) ListApplications(ctx context.Context, req *applicationv1.ListApplicationsRequest) (*applicationv1.ListApplicationsResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pageToken = new(req.GetPageToken())
	}

	apps, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list applications: %v", err)
	}

	resp := &applicationv1.ListApplicationsResponse{}
	for _, app := range apps {
		resp.Applications = append(resp.Applications, app.ToProto())
	}

	if len(apps) > 0 && int32(len(apps)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := apps[len(apps)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateApplication(ctx context.Context, req *applicationv1.UpdateApplicationRequest) (*applicationv1.UpdateApplicationResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
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

	app, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "application not found: %s", id)
	}

	fields := map[string]any{
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

	app, err = a.store.Update(ctx, app, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update application: %v", err)
	}

	return &applicationv1.UpdateApplicationResponse{
		Application: app.ToProto(),
	}, nil
}

func (a *api) DeleteApplication(ctx context.Context, req *applicationv1.DeleteApplicationRequest) (*applicationv1.DeleteApplicationResponse, error) {
	id, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application ID: %v", err)
	}

	result, err := a.store.DeleteCascade(ctx, id, req.GetForce())
	if err != nil {
		if depErr, ok := errors.AsType[*store.HasDependentsError](err); ok {
			return nil, status.Errorf(codes.FailedPrecondition, "%s", depErr.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to delete application: %v", err)
	}

	if result.Environments > 0 || result.Runs > 0 {
		a.logger.Info("force-deleted application",
			zap.String("application_id", id.String()),
			zap.Int64("environments_deleted", result.Environments),
			zap.Int64("runs_deleted", result.Runs),
		)
	}

	return &applicationv1.DeleteApplicationResponse{}, nil
}
