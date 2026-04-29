package runner

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/middleware/httpauth"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/service/encryption"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/service/orchestration"
	"go.admiral.io/admiral/internal/store"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

const (
	Name             = "endpoint.runner"
	defaultTokenName = "default"
	runnerExecScope  = "runner:exec"
	stateReadScope   = "state:read"
	stateWriteScope  = "state:write"
)

var (
	filterColumns     = []string{"name"}
	jobsFilterColumns = []string{"status", "job_type", "run_id"}
)

type api struct {
	runnerv1.UnimplementedRunnerAPIServer

	store           *store.RunnerStore
	tokenStore      *store.AccessTokenStore
	jobStore        *store.JobStore
	revisionStore   *store.RevisionStore
	runStore        *store.RunStore
	componentStore  *store.ComponentStore
	moduleStore     *store.ModuleStore
	sourceStore     *store.SourceStore
	credentialStore *store.CredentialStore
	tokenIssuer     authn.TokenIssuer
	sessionProvider authn.SessionProvider
	objStore        objectstorage.Service
	objBucket       string
	orchestration   *orchestration.Service
	runnersQB       querybuilder.QueryBuilder
	jobsQB          querybuilder.QueryBuilder
	logger          *zap.Logger
	scope           tally.Scope
}

func New(cfg *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}

	runnerStore, err := store.NewRunnerStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	tokenStore, err := store.NewAccessTokenStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	jobStore, err := store.NewJobStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	revisionStore, err := store.NewRevisionStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	runStore, err := store.NewRunStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	componentStore, err := store.NewComponentStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	moduleStore, err := store.NewModuleStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	sourceStore, err := store.NewSourceStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	enc, err := service.GetService[encryption.Service](encryption.Name)
	if err != nil {
		return nil, err
	}
	credentialStore, err := store.NewCredentialStore(db.GormDB(), enc)
	if err != nil {
		return nil, err
	}

	authnService, err := service.GetService[authn.Service](authn.Name)
	if err != nil {
		return nil, err
	}

	objStore, err := service.GetService[objectstorage.Service](objectstorage.Name)
	if err != nil {
		return nil, fmt.Errorf("object storage is required: %w", err)
	}
	objBucket := cfg.Services.ObjectStorage.Bucket

	orch, err := service.GetService[*orchestration.Service](orchestration.Name)
	if err != nil {
		return nil, err
	}

	return &api{
		store:           runnerStore,
		tokenStore:      tokenStore,
		jobStore:        jobStore,
		revisionStore:   revisionStore,
		runStore:        runStore,
		componentStore:  componentStore,
		moduleStore:     moduleStore,
		sourceStore:     sourceStore,
		credentialStore: credentialStore,
		tokenIssuer:     authnService,
		sessionProvider: authnService,
		objStore:        objStore,
		objBucket:       objBucket,
		orchestration:   orch,
		logger:          log.Named(Name),
		scope:           scope.SubScope("runner"),
		runnersQB:       querybuilder.New("runners", filterColumns),
		jobsQB:          querybuilder.New("jobs", jobsFilterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	runnerv1.RegisterRunnerAPIServer(r.GRPCServer(), a)
	if err := r.RegisterJSONGateway(runnerv1.RegisterRunnerAPIHandler); err != nil {
		return err
	}

	withAuth := httpauth.Middleware(httpauth.Config{
		SessionProvider: a.sessionProvider,
		RequiredKind:    new(authn.TokenKindSAT),
	})
	mux := r.HTTPMux()
	mux.Handle("GET "+artifactRoutePattern, withAuth(http.HandlerFunc(a.serveArtifact)))
	mux.Handle("POST "+planFileRoutePattern, withAuth(http.HandlerFunc(a.uploadPlanFile)))
	mux.Handle("GET "+planFileRoutePattern, withAuth(http.HandlerFunc(a.downloadPlanFile)))
	return nil
}

func (a *api) CreateRunner(ctx context.Context, req *runnerv1.CreateRunnerRequest) (*runnerv1.CreateRunnerResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	r := &model.Runner{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Labels:      model.Labels(req.GetLabels()),
		CreatedBy:   claims.Subject,
	}

	r, err = a.store.Create(ctx, r)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create runner: %v", err)
	}

	_, plaintext, err := a.tokenIssuer.CreateToken(
		ctx,
		authn.TokenKindSAT,
		model.AccessTokenBindingTypeRunner,
		defaultTokenName,
		r.Id.String(),
		[]string{runnerExecScope, stateReadScope, stateWriteScope},
		nil,
	)
	if err != nil {
		// Rollback the runner so the caller can retry cleanly rather than
		// ending up with a token-less runner in the tenant.
		_ = a.store.Delete(ctx, r.Id)
		return nil, status.Errorf(codes.Internal, "failed to issue runner token: %v", err)
	}

	return &runnerv1.CreateRunnerResponse{
		Runner:         r.ToProto(),
		PlainTextToken: plaintext,
	}, nil
}

func (a *api) GetRunner(ctx context.Context, req *runnerv1.GetRunnerRequest) (*runnerv1.GetRunnerResponse, error) {
	id, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	r, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", id)
	}

	return &runnerv1.GetRunnerResponse{
		Runner: r.ToProto(),
	}, nil
}

func (a *api) ListRunners(ctx context.Context, req *runnerv1.ListRunnersRequest) (*runnerv1.ListRunnersResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	runners, err := a.store.List(ctx, a.runnersQB.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list runners: %v", err)
	}

	resp := &runnerv1.ListRunnersResponse{}
	for i := range runners {
		resp.Runners = append(resp.Runners, runners[i].ToProto())
	}

	if len(runners) > 0 && int32(len(runners)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := runners[len(runners)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}

	return resp, nil
}

func (a *api) UpdateRunner(ctx context.Context, req *runnerv1.UpdateRunnerRequest) (*runnerv1.UpdateRunnerResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	rProto := req.GetRunner()
	if rProto == nil {
		return nil, status.Error(codes.InvalidArgument, "runner is required")
	}

	id, err := uuid.Parse(rProto.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	existing, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", id)
	}

	fields := map[string]any{
		"updated_at": time.Now(),
	}

	mask := req.GetUpdateMask()
	if mask == nil || len(mask.GetPaths()) == 0 {
		fields["name"] = rProto.GetName()
		fields["description"] = rProto.GetDescription()
		fields["labels"] = model.Labels(rProto.GetLabels())
	} else {
		for _, path := range mask.GetPaths() {
			switch path {
			case "name":
				fields["name"] = rProto.GetName()
			case "description":
				fields["description"] = rProto.GetDescription()
			case "labels":
				fields["labels"] = model.Labels(rProto.GetLabels())
			default:
				return nil, status.Errorf(codes.InvalidArgument, "unsupported update field: %s", path)
			}
		}
	}

	existing, err = a.store.Update(ctx, existing, fields)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update runner: %v", err)
	}

	return &runnerv1.UpdateRunnerResponse{
		Runner: existing.ToProto(),
	}, nil
}

func (a *api) DeleteRunner(ctx context.Context, req *runnerv1.DeleteRunnerRequest) (*runnerv1.DeleteRunnerResponse, error) {
	id, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	if _, err := a.store.Get(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", id)
	}

	if _, err := a.tokenStore.DeleteBySubject(ctx, id.String()); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke runner tokens: %v", err)
	}

	if err := a.store.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete runner: %v", err)
	}

	return &runnerv1.DeleteRunnerResponse{}, nil
}

func (a *api) GetRunnerStatus(ctx context.Context, req *runnerv1.GetRunnerStatusRequest) (*runnerv1.GetRunnerStatusResponse, error) {
	id, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	r, err := a.store.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", id)
	}

	resp := &runnerv1.GetRunnerStatusResponse{
		HealthStatus: model.DeriveHealthStatus(r.LastHeartbeatAt, time.Now()),
	}

	if r.LastStatus != nil {
		resp.Status = r.LastStatus.ToProto()
	}
	if r.LastHeartbeatAt != nil {
		resp.ReportedAt = timestamppb.New(*r.LastHeartbeatAt)
	}

	return resp, nil
}
