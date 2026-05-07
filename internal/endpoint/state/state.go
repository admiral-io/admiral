package state

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 used for content fingerprint, not security
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/middleware/httpauth"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/store"
)

const (
	Name = "endpoint.state"

	stateReadScope  = "state:read"
	stateWriteScope = "state:write"

	stateRoutePattern = "/api/v1/state/{component_id}/env/{environment_id}"
	lockRoutePattern  = "/api/v1/state/{component_id}/env/{environment_id}/lock"
)

type stateStore interface {
	GetLatest(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformState, error)
	Create(ctx context.Context, st *model.TerraformState) (*model.TerraformState, error)
	SetStoragePath(ctx context.Context, id uuid.UUID, path string) error
	GetLock(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformStateLock, error)
	AcquireLock(ctx context.Context, lock *model.TerraformStateLock) (*model.TerraformStateLock, error)
	ReleaseLock(ctx context.Context, componentID, environmentID uuid.UUID, lockID string) error
}

type componentGetter interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Component, error)
}

type environmentGetter interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Environment, error)
}

type requestContext struct {
	claims        *authn.Claims
	componentID   uuid.UUID
	environmentID uuid.UUID
}

type api struct {
	stateStore       stateStore
	componentStore   componentGetter
	environmentStore environmentGetter
	sessionProvider  authn.SessionProvider
	objStore         objectstorage.Service
	objBucket        string
	logger           *zap.Logger
	scope            tally.Scope
}

func New(cfg *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}

	stateStore, err := store.NewTerraformStateStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	componentStore, err := store.NewComponentStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	environmentStore, err := store.NewEnvironmentStore(db.GormDB())
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

	return &api{
		stateStore:       stateStore,
		componentStore:   componentStore,
		environmentStore: environmentStore,
		sessionProvider:  authnService,
		objStore:         objStore,
		objBucket:        objBucket,
		logger:           log.Named(Name),
		scope:            scope.SubScope("state"),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	withAuth := httpauth.Middleware(httpauth.Config{
		SessionProvider: a.sessionProvider,
		ScopeForMethod:  scopeForStateMethod,
		AllowBasicAuth:  true,
	})
	mux := r.HTTPMux()
	mux.Handle("GET "+stateRoutePattern, withAuth(http.HandlerFunc(a.handleGetState)))
	mux.Handle("POST "+stateRoutePattern, withAuth(http.HandlerFunc(a.handlePostState)))
	mux.Handle("LOCK "+lockRoutePattern, withAuth(http.HandlerFunc(a.handleLock)))
	mux.Handle("UNLOCK "+lockRoutePattern, withAuth(http.HandlerFunc(a.handleUnlock)))
	return nil
}

// resolveRequest parses path params and validates that the component and
// environment exist and belong to the same application.
// On failure it writes the HTTP error and returns nil.
func (a *api) resolveRequest(w http.ResponseWriter, r *http.Request) *requestContext {
	claims, err := authn.ClaimsFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}

	componentID, err := uuid.Parse(r.PathValue("component_id"))
	if err != nil {
		http.Error(w, "invalid component_id", http.StatusBadRequest)
		return nil
	}
	environmentID, err := uuid.Parse(r.PathValue("environment_id"))
	if err != nil {
		http.Error(w, "invalid environment_id", http.StatusBadRequest)
		return nil
	}

	ctx := r.Context()
	comp, err := a.componentStore.Get(ctx, componentID)
	if err != nil {
		http.Error(w, "component not found", http.StatusNotFound)
		return nil
	}
	env, err := a.environmentStore.Get(ctx, environmentID)
	if err != nil {
		http.Error(w, "environment not found", http.StatusNotFound)
		return nil
	}
	if comp.ApplicationId != env.ApplicationId {
		http.Error(w, "component and environment belong to different applications", http.StatusBadRequest)
		return nil
	}

	return &requestContext{
		claims:        claims,
		componentID:   componentID,
		environmentID: environmentID,
	}
}

func storagePath(componentID, environmentID uuid.UUID, stateID uuid.UUID) string {
	return fmt.Sprintf("state/%s/%s/%s.tfstate", componentID, environmentID, stateID)
}

// handleGetState implements GET -- returns the current state.
// Terraform expects 200 with the state body, or 204/404 if no state exists.
func (a *api) handleGetState(w http.ResponseWriter, r *http.Request) {
	rc := a.resolveRequest(w, r)
	if rc == nil {
		return
	}

	ctx := r.Context()
	st, err := a.stateStore.GetLatest(ctx, rc.componentID, rc.environmentID)
	if err != nil {
		a.logger.Error("get state failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if st == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	data, err := a.objStore.GetObject(ctx, a.objBucket, st.StoragePath)
	if err != nil {
		a.logger.Error("get state blob failed",
			zap.String("path", st.StoragePath), zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handlePostState implements POST -- writes a new state version.
// Terraform sends the full state JSON as the request body.
// If the state is locked, the request must include a matching lock ID.
func (a *api) handlePostState(w http.ResponseWriter, r *http.Request) {
	rc := a.resolveRequest(w, r)
	if rc == nil {
		return
	}

	ctx := r.Context()

	body, err := io.ReadAll(io.LimitReader(r.Body, objectstorage.MaxObjectSize+1))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > objectstorage.MaxObjectSize {
		http.Error(w, "state too large", http.StatusRequestEntityTooLarge)
		return
	}

	var stateDoc struct {
		Serial  int64  `json:"serial"`
		Lineage string `json:"lineage"`
	}
	if err := json.Unmarshal(body, &stateDoc); err != nil {
		http.Error(w, "invalid state JSON", http.StatusBadRequest)
		return
	}

	// If locked, verify the caller holds the lock.
	lockID := r.URL.Query().Get("ID")
	lock, err := a.stateStore.GetLock(ctx, rc.componentID, rc.environmentID)
	if err != nil {
		a.logger.Error("check lock failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if lock != nil && lock.LockId != lockID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusLocked)
		_ = json.NewEncoder(w).Encode(lockToJSON(lock))
		return
	}

	hash := md5.Sum(body) //nolint:gosec
	md5Hex := hex.EncodeToString(hash[:])

	st := &model.TerraformState{
		ComponentId:   rc.componentID,
		EnvironmentId: rc.environmentID,
		Serial:        stateDoc.Serial,
		Lineage:       stateDoc.Lineage,
		ContentLength: int64(len(body)),
		ContentMD5:    md5Hex,
		LockId:        lockID,
		CreatedBy:     rc.claims.Subject,
	}
	st, err = a.stateStore.Create(ctx, st)
	if err != nil {
		a.logger.Error("create state record failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	path := storagePath(rc.componentID, rc.environmentID, st.Id)
	if err := a.objStore.PutObject(ctx, a.objBucket, path, body); err != nil {
		a.logger.Error("put state blob failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := a.stateStore.SetStoragePath(ctx, st.Id, path); err != nil {
		a.logger.Error("update storage path failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func scopeForStateMethod(method string) string {
	if method == http.MethodGet {
		return stateReadScope
	}
	return stateWriteScope
}
