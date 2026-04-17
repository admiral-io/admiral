package runner

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
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/store"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

const (
	Name             = "endpoint.runner"
	defaultTokenName = "default"
	runnerExecScope  = "runner:exec"
)

var (
	filterColumns     = []string{"name", "kind"}
	jobsFilterColumns = []string{"status", "job_type", "deployment_id"}
)

type api struct {
	runnerv1.UnimplementedRunnerAPIServer

	store           *store.RunnerStore
	tokenStore      *store.AccessTokenStore
	jobStore        *store.JobStore
	revisionStore   *store.RevisionStore
	deploymentStore *store.DeploymentStore
	componentStore  *store.ComponentStore
	moduleStore     *store.ModuleStore
	sourceStore     *store.SourceStore
	credentialStore *store.CredentialStore
	variableStore   *store.VariableStore
	tokenIssuer     authn.TokenIssuer
	sessionProvider authn.SessionProvider
	objStore        objectstorage.Service
	objBucket       string
	qb              querybuilder.QueryBuilder
	jobsQB          querybuilder.QueryBuilder
	logger          *zap.Logger
	scope           tally.Scope
}

func New(cfg *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
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
	deploymentStore, err := store.NewDeploymentStore(db.GormDB())
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
	credentialStore, err := store.NewCredentialStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	variableStore, err := store.NewVariableStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	authnService, err := service.GetService[authn.Service]("service.authn")
	if err != nil {
		return nil, err
	}

	objStore, err := service.GetService[objectstorage.Service](objectstorage.Name)
	if err != nil {
		return nil, fmt.Errorf("object storage is required: %w", err)
	}
	objBucket := cfg.Services.ObjectStorage.Bucket

	return &api{
		store:           runnerStore,
		tokenStore:      tokenStore,
		jobStore:        jobStore,
		revisionStore:   revisionStore,
		deploymentStore: deploymentStore,
		componentStore:  componentStore,
		moduleStore:     moduleStore,
		sourceStore:     sourceStore,
		credentialStore: credentialStore,
		variableStore:   variableStore,
		tokenIssuer:     authnService,
		sessionProvider: authnService,
		objStore:        objStore,
		objBucket:       objBucket,
		logger:          log.Named(Name),
		scope:           scope.SubScope("runner"),
		qb:              querybuilder.New(filterColumns),
		jobsQB:          querybuilder.New(jobsFilterColumns),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	runnerv1.RegisterRunnerAPIServer(r.GRPCServer(), a)
	if err := r.RegisterJSONGateway(runnerv1.RegisterRunnerAPIHandler); err != nil {
		return err
	}
	r.HTTPMux().HandleFunc("GET "+artifactRoutePattern, a.serveArtifact)
	r.HTTPMux().HandleFunc("POST "+planFileRoutePattern, a.uploadPlanFile)
	r.HTTPMux().HandleFunc("GET "+planFileRoutePattern, a.downloadPlanFile)
	return nil
}

func (a *api) CreateRunner(ctx context.Context, req *runnerv1.CreateRunnerRequest) (*runnerv1.CreateRunnerResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	kind := model.RunnerKindFromProto(req.GetKind())
	if kind == "" {
		return nil, status.Error(codes.InvalidArgument, "runner kind is required")
	}

	r := &model.Runner{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Kind:        kind,
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
		[]string{runnerExecScope},
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

	runners, err := a.store.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
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

func (a *api) CreateRunnerToken(ctx context.Context, req *runnerv1.CreateRunnerTokenRequest) (*runnerv1.CreateRunnerTokenResponse, error) {
	runnerID, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	var expiry *time.Duration
	if req.ExpiresAt != nil {
		d := time.Until(req.GetExpiresAt().AsTime())
		if d <= 0 {
			return nil, status.Error(codes.InvalidArgument, "expires_at must be in the future")
		}
		expiry = &d
	}

	token, plaintext, err := a.tokenIssuer.CreateToken(
		ctx,
		authn.TokenKindSAT,
		model.AccessTokenBindingTypeRunner,
		req.GetName(),
		runnerID.String(),
		[]string{runnerExecScope},
		expiry,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create runner token: %v", err)
	}

	return &runnerv1.CreateRunnerTokenResponse{
		AccessToken:    token.ToProto(),
		PlainTextToken: plaintext,
	}, nil
}

func (a *api) ListRunnerTokens(ctx context.Context, req *runnerv1.ListRunnerTokensRequest) (*runnerv1.ListRunnerTokensResponse, error) {
	runnerID, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	tokens, err := a.tokenStore.ListBySubject(ctx, runnerID.String(), string(model.AccessTokenKindSAT))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list runner tokens: %v", err)
	}

	resp := &runnerv1.ListRunnerTokensResponse{}
	for i := range tokens {
		resp.AccessTokens = append(resp.AccessTokens, tokens[i].ToProto())
	}
	return resp, nil
}

func (a *api) GetRunnerToken(ctx context.Context, req *runnerv1.GetRunnerTokenRequest) (*runnerv1.GetRunnerTokenResponse, error) {
	token, err := a.runnerTokenOrErr(ctx, req.GetRunnerId(), req.GetTokenId())
	if err != nil {
		return nil, err
	}
	return &runnerv1.GetRunnerTokenResponse{
		AccessToken: token.ToProto(),
	}, nil
}

func (a *api) RevokeRunnerToken(ctx context.Context, req *runnerv1.RevokeRunnerTokenRequest) (*runnerv1.RevokeRunnerTokenResponse, error) {
	token, err := a.runnerTokenOrErr(ctx, req.GetRunnerId(), req.GetTokenId())
	if err != nil {
		return nil, err
	}

	if err := a.tokenStore.Delete(ctx, token.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke runner token: %v", err)
	}

	token.Status = model.AccessTokenStatusRevoked
	return &runnerv1.RevokeRunnerTokenResponse{
		AccessToken: token.ToProto(),
	}, nil
}

func (a *api) Heartbeat(ctx context.Context, req *runnerv1.HeartbeatRequest) (*runnerv1.HeartbeatResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	if req.GetStatus() == nil {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	instanceID, err := uuid.Parse(req.GetInstanceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid instance_id: %v", err)
	}

	snapshot := model.RunnerStatusFromProto(req.GetStatus())

	if err := a.store.UpdateHeartbeat(ctx, runnerID, snapshot, instanceID, time.Now()); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record heartbeat: %v", err)
	}

	return &runnerv1.HeartbeatResponse{
		Ack:                  true,
		NextHeartbeatSeconds: int32(model.HeartbeatInterval.Seconds()),
	}, nil
}

func (a *api) runnerTokenOrErr(ctx context.Context, runnerIDStr, tokenID string) (*model.AccessToken, error) {
	runnerID, err := uuid.Parse(runnerIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	token, err := a.tokenStore.Get(ctx, tokenID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", tokenID)
	}

	if token.Subject != runnerID.String() || token.Kind != model.AccessTokenKindSAT {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", tokenID)
	}

	return token, nil
}

func runnerIDFromClaims(ctx context.Context) (uuid.UUID, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	if claims.Kind != string(authn.TokenKindSAT) {
		return uuid.Nil, status.Error(codes.PermissionDenied, "runner SAT required")
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, status.Error(codes.Internal, "invalid subject in token")
	}

	return id, nil
}
