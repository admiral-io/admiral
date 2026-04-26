package state

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/store"
)

const (
	Name = "endpoint.state"

	stateRoutePattern = "/api/v1/state/{component_id}/env/{environment_id}"
	lockRoutePattern  = "/api/v1/state/{component_id}/env/{environment_id}/lock"
)

// stateStore defines the state persistence operations needed by this endpoint.
type stateStore interface {
	GetLatest(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformState, error)
	Create(ctx context.Context, st *model.TerraformState) (*model.TerraformState, error)
	SetStoragePath(ctx context.Context, id uuid.UUID, path string) error
	GetLock(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformStateLock, error)
	AcquireLock(ctx context.Context, lock *model.TerraformStateLock) (*model.TerraformStateLock, error)
	ReleaseLock(ctx context.Context, componentID, environmentID uuid.UUID, lockID string) error
}

// componentGetter looks up a component by ID.
type componentGetter interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Component, error)
}

// environmentGetter looks up an environment by ID.
type environmentGetter interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Environment, error)
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
	mux := r.HTTPMux()
	mux.HandleFunc("GET "+stateRoutePattern, a.withAuth(a.handleGetState))
	mux.HandleFunc("POST "+stateRoutePattern, a.withAuth(a.handlePostState))
	mux.HandleFunc("LOCK "+lockRoutePattern, a.withAuth(a.handleLock))
	mux.HandleFunc("UNLOCK "+lockRoutePattern, a.withAuth(a.handleUnlock))
	return nil
}

// requestContext bundles the validated path params for a state request.
type requestContext struct {
	claims        *authn.Claims
	componentID   uuid.UUID
	environmentID uuid.UUID
}

// resolveRequest parses path params and validates that the component and
// environment exist and belong to the same application.
// On failure it writes the HTTP error and returns nil.
func (a *api) resolveRequest(w http.ResponseWriter, r *http.Request) *requestContext {
	claims := claimsFromContext(r.Context())

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

// storagePath returns the object storage key for a state blob.
func storagePath(componentID, environmentID uuid.UUID, stateID uuid.UUID) string {
	return fmt.Sprintf("state/%s/%s/%s.tfstate", componentID, environmentID, stateID)
}