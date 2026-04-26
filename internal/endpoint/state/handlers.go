package state

import (
	"crypto/md5" //nolint:gosec // MD5 used for content fingerprint, not security
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/store"
)

// handleGetState implements GET - returns the current state.
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

// handlePostState implements POST - writes a new state version.
// Terraform sends the full state JSON as the request body.
// If the state is locked, the request must include a matching lock ID.
func (a *api) handlePostState(w http.ResponseWriter, r *http.Request) {
	rc := a.resolveRequest(w, r)
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
	rc := a.resolveRequest(w, r)
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
	if errors.Is(err, store.ErrLockConflict) {
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
	rc := a.resolveRequest(w, r)
	if rc == nil {
		return
	}

	var lockInfo lockJSON
	if err := json.NewDecoder(r.Body).Decode(&lockInfo); err != nil {
		http.Error(w, "invalid lock info", http.StatusBadRequest)
		return
	}

	err := a.stateStore.ReleaseLock(r.Context(), rc.componentID, rc.environmentID, lockInfo.ID)
	if errors.Is(err, store.ErrLockConflict) {
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