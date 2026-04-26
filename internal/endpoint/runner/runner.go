package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
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
	stateScope       = "state:rw"
)

var (
	filterColumns     = []string{"name"}
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
	tokenIssuer     authn.TokenIssuer
	sessionProvider authn.SessionProvider
	objStore        objectstorage.Service
	objBucket       string
	orchestration   *orchestration.Service
	qb              querybuilder.QueryBuilder
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
	deploymentStore, err := store.NewDeploymentStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	envStore, err := store.NewEnvironmentStore(db.GormDB())
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
	variableStore, err := store.NewVariableStore(db.GormDB())
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
	baseURL := strings.TrimRight(cfg.Server.ExternalURL, "/")

	orch := orchestration.New(
		jobStore,
		revisionStore,
		deploymentStore,
		envStore,
		variableStore,
		objStore,
		objBucket,
		baseURL,
		log,
	)

	return &api{
		store:         runnerStore,
		tokenStore:    tokenStore,
		jobStore:      jobStore,
		revisionStore: revisionStore,
		deploymentStore: deploymentStore,
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
		qb:              querybuilder.New("runners", filterColumns),
		jobsQB:          querybuilder.New("jobs", jobsFilterColumns),
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
