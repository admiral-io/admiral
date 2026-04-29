package state

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/store"
)

// handleLock implements LOCK -- acquires a state lock.
// Terraform sends a JSON lock info body.
// Returns 200 on success, 423 if already locked (with existing lock info).
func (a *api) handleLock(w http.ResponseWriter, r *http.Request) {
	rc := a.resolveRequest(w, r)
	if rc == nil {
		return
	}

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

// handleUnlock implements UNLOCK -- releases a state lock.
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
