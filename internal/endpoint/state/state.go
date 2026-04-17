package state

import (
	"crypto/md5" //nolint:gosec // MD5 used for content fingerprint, not security
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

type api struct {
	stateStore      *store.TerraformStateStore
	componentStore  *store.ComponentStore
	environmentStore *store.EnvironmentStore
	sessionProvider authn.SessionProvider
	objStore        objectstorage.Service
	objBucket       string
	logger          *zap.Logger
	scope           tally.Scope
}

func New(cfg *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
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
	r.HTTPMux().HandleFunc("GET "+stateRoutePattern, a.handleGetState)
	r.HTTPMux().HandleFunc("POST "+stateRoutePattern, a.handlePostState)
	r.HTTPMux().HandleFunc("LOCK "+lockRoutePattern, a.handleLock)
	r.HTTPMux().HandleFunc("UNLOCK "+lockRoutePattern, a.handleUnlock)
	return nil
}

// authenticate extracts Basic auth credentials and verifies the password
// as a PAT or SAT via the session provider.  Returns the verified claims.
func (a *api) authenticate(r *http.Request) (*authn.Claims, error) {
	_, password, ok := r.BasicAuth()
	if !ok {
		// Fall back to Bearer token.
		token, err := extractBearer(r.Header.Get("Authorization"))
		if err != nil {
			return nil, fmt.Errorf("authentication required")
		}
		return a.sessionProvider.Verify(r.Context(), token)
	}
	return a.sessionProvider.Verify(r.Context(), password)
}

// requestContext bundles the validated result of authenticating and parsing
// a state request so handlers don't repeat boilerplate.
type requestContext struct {
	claims        *authn.Claims
	componentID   uuid.UUID
	environmentID uuid.UUID
}

// authorizeRequest authenticates, parses path params, and validates that the
// component and environment exist and belong to the same application.
// On failure it writes the HTTP error and returns nil.
func (a *api) authorizeRequest(w http.ResponseWriter, r *http.Request) *requestContext {
	claims, err := a.authenticate(r)
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

// storagePath returns the object storage key for a state blob.
func storagePath(componentID, environmentID uuid.UUID, stateID uuid.UUID) string {
	return fmt.Sprintf("state/%s/%s/%s.tfstate", componentID, environmentID, stateID)
}

// handleGetState implements GET - returns the current state.
// Terraform expects 200 with the state body, or 204/404 if no state exists.
func (a *api) handleGetState(w http.ResponseWriter, r *http.Request) {
	rc := a.authorizeRequest(w, r)
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

// handlePostState implements POST - writes a new state version.
// Terraform sends the full state JSON as the request body.
// If the state is locked, the request must include a matching lock ID.
func (a *api) handlePostState(w http.ResponseWriter, r *http.Request) {
	rc := a.authorizeRequest(w, r)
	if rc == nil {
		return
	}

	ctx := r.Context()

	// Read the state body.
	body, err := io.ReadAll(io.LimitReader(r.Body, objectstorage.MaxObjectSize+1))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > objectstorage.MaxObjectSize {
		http.Error(w, "state too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Extract serial and lineage from the state JSON.
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

	// Compute content hash.
	hash := md5.Sum(body) //nolint:gosec
	md5Hex := hex.EncodeToString(hash[:])

	// Create the metadata record to get the ID for the storage path.
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

	// Store the blob.
	path := storagePath(rc.componentID, rc.environmentID, st.Id)
	if err := a.objStore.PutObject(ctx, a.objBucket, path, body); err != nil {
		a.logger.Error("put state blob failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Update the metadata record with the storage path.
	if err := a.stateStore.SetStoragePath(ctx, st.Id, path); err != nil {
		a.logger.Error("update storage path failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleLock implements LOCK - acquires a state lock.
// Terraform sends a JSON lock info body.
// Returns 200 on success, 423 if already locked (with existing lock info).
func (a *api) handleLock(w http.ResponseWriter, r *http.Request) {
	rc := a.authorizeRequest(w, r)
	if rc == nil {
		return
	}

	// Parse lock info from Terraform.
	var lockInfo lockJSON
	if err := json.NewDecoder(r.Body).Decode(&lockInfo); err != nil {
		http.Error(w, "invalid lock info", http.StatusBadRequest)
		return
	}
	if lockInfo.ID == "" {
		http.Error(w, "lock ID is required", http.StatusBadRequest)
		return
	}

	lock := &model.TerraformStateLock{
		ComponentId:   rc.componentID,
		EnvironmentId: rc.environmentID,
		LockId:        lockInfo.ID,
		Operation:     lockInfo.Operation,
		Who:           lockInfo.Who,
		Info:          lockInfo.Info,
		Version:       lockInfo.Version,
		Path:          lockInfo.Path,
	}

	existing, err := a.stateStore.AcquireLock(r.Context(), lock)
	if err == store.ErrLockConflict {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusLocked)
		_ = json.NewEncoder(w).Encode(lockToJSON(existing))
		return
	}
	if err != nil {
		a.logger.Error("acquire lock failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleUnlock implements UNLOCK - releases a state lock.
// Terraform sends the same lock info body it used to acquire.
// Returns 200 on success, 423 if the lock is held by a different ID.
func (a *api) handleUnlock(w http.ResponseWriter, r *http.Request) {
	rc := a.authorizeRequest(w, r)
	if rc == nil {
		return
	}

	var lockInfo lockJSON
	if err := json.NewDecoder(r.Body).Decode(&lockInfo); err != nil {
		http.Error(w, "invalid lock info", http.StatusBadRequest)
		return
	}

	err := a.stateStore.ReleaseLock(r.Context(), rc.componentID, rc.environmentID, lockInfo.ID)
	if err == store.ErrLockConflict {
		existing, loadErr := a.stateStore.GetLock(r.Context(), rc.componentID, rc.environmentID)
		if loadErr != nil {
			a.logger.Error("load lock for conflict response failed", zap.Error(loadErr))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusLocked)
		_ = json.NewEncoder(w).Encode(lockToJSON(existing))
		return
	}
	if err != nil {
		a.logger.Error("release lock failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// lockJSON matches Terraform's lock info JSON structure.
type lockJSON struct {
	ID        string `json:"ID"`
	Operation string `json:"Operation"`
	Info      string `json:"Info"`
	Who       string `json:"Who"`
	Version   string `json:"Version"`
	Created   string `json:"Created"`
	Path      string `json:"Path"`
}

func lockToJSON(lock *model.TerraformStateLock) lockJSON {
	return lockJSON{
		ID:        lock.LockId,
		Operation: lock.Operation,
		Info:      lock.Info,
		Who:       lock.Who,
		Version:   lock.Version,
		Created:   lock.CreatedAt.Format("2006-01-02T15:04:05.999999Z"),
		Path:      lock.Path,
	}
}

func extractBearer(header string) (string, error) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", fmt.Errorf("missing or malformed Authorization header")
	}
	if fields[1] == "" {
		return "", fmt.Errorf("empty bearer token")
	}
	return fields[1], nil
}
