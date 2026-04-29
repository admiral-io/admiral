package run

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

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
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/service/orchestration"
	"go.admiral.io/admiral/internal/store"
	runv1 "go.admiral.io/sdk/proto/admiral/run/v1"
)

const Name = "endpoint.run"

var filterColumns = []string{"application_id", "environment_id", "status", "change_set_id"}

type api struct {
	runv1.UnimplementedRunAPIServer

	runStore      *store.RunStore
	revisionStore *store.RevisionStore
	orch          *orchestration.Service
	objStore      objectstorage.Service
	objBucket     string
	qb            querybuilder.QueryBuilder
	logger        *zap.Logger
	scope         tally.Scope
}

func New(cfg *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}

	runStore, err := store.NewRunStore(db.GormDB())
	if err != nil {
		return nil, err
	}
	revisionStore, err := store.NewRevisionStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	objStore, err := service.GetService[objectstorage.Service](objectstorage.Name)
	if err != nil {
		return nil, fmt.Errorf("object storage is required: %w", err)
	}

	orch, err := service.GetService[*orchestration.Service](orchestration.Name)
	if err != nil {
		return nil, err
	}

	return &api{
		runStore:      runStore,
		revisionStore: revisionStore,
		orch:          orch,
		objStore:      objStore,
		objBucket:     cfg.Services.ObjectStorage.Bucket,
		logger:        log.Named(Name),
		scope:         scope.SubScope("run"),
		qb:            querybuilder.New("runs", filterColumns),
	}, nil
}

const planOutputRoutePattern = "/api/v1/runs/{run_id}/revisions/{revision_id}/plan"

func (a *api) Register(r endpoint.Registrar) error {
	runv1.RegisterRunAPIServer(r.GRPCServer(), a)
	r.HTTPMux().HandleFunc("GET "+planOutputRoutePattern, a.servePlanOutput)
	return r.RegisterJSONGateway(runv1.RegisterRunAPIHandler)
}

func (a *api) CreateRun(ctx context.Context, req *runv1.CreateRunRequest) (*runv1.CreateRunResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if req.GetDestroy() {
		return nil, status.Error(codes.Unimplemented, "destroy runs are not yet supported")
	}

	appID, err := uuid.Parse(req.GetApplicationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid application_id: %v", err)
	}
	envID, err := uuid.Parse(req.GetEnvironmentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid environment_id: %v", err)
	}
	var sourceRunID *uuid.UUID
	if raw := req.GetSourceRunId(); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid source_run_id: %v", err)
		}
		sourceRunID = &id
	}
	var changeSetID *uuid.UUID
	if raw := req.GetChangeSetId(); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid change_set_id: %v", err)
		}
		changeSetID = &id
	}

	run, err := a.orch.CreateRun(ctx, orchestration.CreateRunParams{
		ApplicationID: appID,
		EnvironmentID: envID,
		TriggeredBy:   claims.Subject,
		Message:       req.GetMessage(),
		SourceRunID:   sourceRunID,
		ChangeSetID:   changeSetID,
	})
	if err != nil {
		return nil, err
	}
	return &runv1.CreateRunResponse{Run: a.loadRunProto(ctx, run)}, nil
}

func (a *api) GetRun(ctx context.Context, req *runv1.GetRunRequest) (*runv1.GetRunResponse, error) {
	id, err := uuid.Parse(req.GetRunId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid run_id: %v", err)
	}
	run, err := a.runStore.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "run not found: %s", id)
	}
	return &runv1.GetRunResponse{Run: a.loadRunProto(ctx, run)}, nil
}

func (a *api) ListRuns(ctx context.Context, req *runv1.ListRunsRequest) (*runv1.ListRunsResponse, error) {
	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	runs, err := a.runStore.List(ctx, a.qb.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list runs: %v", err)
	}

	resp := &runv1.ListRunsResponse{}
	for i := range runs {
		resp.Runs = append(resp.Runs, runs[i].ToProto(nil))
	}

	if len(runs) > 0 && int32(len(runs)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := runs[len(runs)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}
	return resp, nil
}

func (a *api) ApplyRun(ctx context.Context, req *runv1.ApplyRunRequest) (*runv1.ApplyRunResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	runID, err := uuid.Parse(req.GetRunId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid run_id: %v", err)
	}
	run, err := a.orch.ApplyRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return &runv1.ApplyRunResponse{Run: a.loadRunProto(ctx, run)}, nil
}

func (a *api) CancelRun(ctx context.Context, req *runv1.CancelRunRequest) (*runv1.CancelRunResponse, error) {
	if _, err := authn.ClaimsFromContext(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	runID, err := uuid.Parse(req.GetRunId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid run_id: %v", err)
	}
	run, err := a.orch.CancelRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return &runv1.CancelRunResponse{Run: a.loadRunProto(ctx, run)}, nil
}

func (a *api) GetRevision(ctx context.Context, req *runv1.GetRevisionRequest) (*runv1.GetRevisionResponse, error) {
	runID, err := uuid.Parse(req.GetRunId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid run_id: %v", err)
	}
	revID, err := uuid.Parse(req.GetRevisionId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid revision_id: %v", err)
	}
	rev, err := a.revisionStore.Get(ctx, revID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "revision not found: %s", revID)
	}
	if rev.RunId != runID {
		return nil, status.Errorf(codes.NotFound, "revision not found: %s", revID)
	}
	return &runv1.GetRevisionResponse{Revision: rev.ToProto()}, nil
}

func (a *api) ListRevisions(ctx context.Context, req *runv1.ListRevisionsRequest) (*runv1.ListRevisionsResponse, error) {
	runID, err := uuid.Parse(req.GetRunId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid run_id: %v", err)
	}
	revisions, err := a.revisionStore.ListByRun(ctx, runID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list revisions: %v", err)
	}
	resp := &runv1.ListRevisionsResponse{}
	for i := range revisions {
		resp.Revisions = append(resp.Revisions, revisions[i].ToProto())
	}
	return resp, nil
}

func (a *api) servePlanOutput(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(r.PathValue("run_id"))
	if err != nil {
		http.Error(w, "invalid run_id", http.StatusBadRequest)
		return
	}
	revID, err := uuid.Parse(r.PathValue("revision_id"))
	if err != nil {
		http.Error(w, "invalid revision_id", http.StatusBadRequest)
		return
	}

	rev, err := a.revisionStore.Get(r.Context(), revID)
	if err != nil {
		http.Error(w, "revision not found", http.StatusNotFound)
		return
	}
	if rev.RunId != runID {
		http.Error(w, "revision not found", http.StatusNotFound)
		return
	}
	if rev.PlanOutputKey == "" {
		http.Error(w, "no plan output available", http.StatusNotFound)
		return
	}
	data, err := a.objStore.GetObject(r.Context(), a.objBucket, rev.PlanOutputKey)
	if err != nil {
		a.logger.Error("failed to read plan output from object storage",
			zap.String("revision_id", revID.String()),
			zap.String("key", rev.PlanOutputKey),
			zap.Error(err))
		http.Error(w, "failed to read plan output", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		a.logger.Warn("failed to write plan output response",
			zap.String("revision_id", revID.String()),
			zap.Error(err))
	}
}

func (a *api) loadRunProto(ctx context.Context, run *model.Run) *runv1.Run {
	revisions, err := a.revisionStore.ListByRun(ctx, run.Id)
	if err != nil {
		a.logger.Warn("failed to load revisions for summary", zap.String("run_id", run.Id.String()), zap.Error(err))
		return model.BuildRunProto(run, nil)
	}
	return model.BuildRunProto(run, revisions)
}
