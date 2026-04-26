package state

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/store"
)

func validClaims() *authn.Claims {
	return &authn.Claims{
		Subject: uuid.New().String(),
		Kind:    "sat",
		Scopes:  []string{"state:rw"},
	}
}

func withClaims(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), claimsKey{}, validClaims()))
}

func testIDs() (compID, envID, appID uuid.UUID) {
	return uuid.New(), uuid.New(), uuid.New()
}

func defaultComponentStore(compID, appID uuid.UUID) *mockComponentStore {
	return &mockComponentStore{
		getFunc: func(ctx context.Context, id uuid.UUID) (*model.Component, error) {
			return &model.Component{Id: compID, ApplicationId: appID}, nil
		},
	}
}

func defaultEnvironmentStore(envID, appID uuid.UUID) *mockEnvironmentStore {
	return &mockEnvironmentStore{
		getFunc: func(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
			return &model.Environment{Id: envID, ApplicationId: appID}, nil
		},
	}
}

func TestExtractBearer(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{name: "valid bearer", header: "Bearer adms_abc123", want: "adms_abc123"},
		{name: "case insensitive", header: "bearer adms_abc123", want: "adms_abc123"},
		{name: "empty header", header: "", wantErr: true},
		{name: "missing token", header: "Bearer", wantErr: true},
		{name: "wrong scheme", header: "Basic dXNlcjpwYXNz", wantErr: true},
		{name: "too many parts", header: "Bearer token extra", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractBearer(tc.header)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestClaimsFromContext(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		claims := &authn.Claims{Subject: "user-1", Scopes: []string{"state:rw"}}
		ctx := context.WithValue(context.Background(), claimsKey{}, claims)
		got := claimsFromContext(ctx)
		require.NotNil(t, got)
		assert.Equal(t, "user-1", got.Subject)
	})

	t.Run("missing", func(t *testing.T) {
		assert.Nil(t, claimsFromContext(context.Background()))
	})

	t.Run("wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), claimsKey{}, "not-claims")
		assert.Nil(t, claimsFromContext(ctx))
	})
}

func TestStoragePath(t *testing.T) {
	compID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	envID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	stateID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	got := storagePath(compID, envID, stateID)
	assert.Equal(t,
		"state/11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/33333333-3333-3333-3333-333333333333.tfstate",
		got)
}

func TestLockToJSON(t *testing.T) {
	ts := time.Date(2026, 4, 26, 14, 30, 0, 0, time.UTC)
	lock := &model.TerraformStateLock{
		LockId:    "lock-123",
		Operation: "OperationTypePlan",
		Who:       "user@host",
		Version:   "1.9.0",
		CreatedAt: ts,
	}

	got := lockToJSON(lock)
	assert.Equal(t, "lock-123", got.ID)
	assert.Equal(t, "OperationTypePlan", got.Operation)
	assert.Equal(t, "user@host", got.Who)
	assert.Equal(t, "1.9.0", got.Version)
	assert.Equal(t, "2026-04-26T14:30:00Z", got.Created)
}

func TestWithAuth(t *testing.T) {
	t.Run("no auth header", func(t *testing.T) {
		a := &api{sessionProvider: &mockSessionProvider{
			verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
				t.Fatal("verify should not be called")
				return nil, nil
			},
		}}

		handler := a.withAuth(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		})

		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		a := &api{sessionProvider: &mockSessionProvider{
			verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
				return nil, fmt.Errorf("invalid token")
			},
		}}

		handler := a.withAuth(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bad")
		rec := httptest.NewRecorder()
		handler(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing scope", func(t *testing.T) {
		a := &api{sessionProvider: &mockSessionProvider{
			verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
				return &authn.Claims{Subject: "u", Scopes: []string{"runner:exec"}}, nil
			},
		}}

		handler := a.withAuth(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer valid")
		rec := httptest.NewRecorder()
		handler(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("bearer success", func(t *testing.T) {
		a := &api{sessionProvider: &mockSessionProvider{
			verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
				assert.Equal(t, "adms_tok", cred)
				return validClaims(), nil
			},
		}}

		var called bool
		handler := a.withAuth(func(w http.ResponseWriter, r *http.Request) {
			called = true
			require.NotNil(t, claimsFromContext(r.Context()))
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer adms_tok")
		rec := httptest.NewRecorder()
		handler(rec, req)
		assert.True(t, called)
	})

	t.Run("basic auth success", func(t *testing.T) {
		a := &api{sessionProvider: &mockSessionProvider{
			verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
				assert.Equal(t, "adms_tok", cred)
				return validClaims(), nil
			},
		}}

		var called bool
		handler := a.withAuth(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("", "adms_tok")
		rec := httptest.NewRecorder()
		handler(rec, req)
		assert.True(t, called)
	})
}

func TestResolveRequest(t *testing.T) {
	t.Run("invalid component ID", func(t *testing.T) {
		a := &api{}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("component_id", "not-a-uuid")
		req.SetPathValue("environment_id", uuid.New().String())
		rec := httptest.NewRecorder()

		assert.Nil(t, a.resolveRequest(rec, withClaims(req)))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid environment ID", func(t *testing.T) {
		a := &api{}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("component_id", uuid.New().String())
		req.SetPathValue("environment_id", "not-a-uuid")
		rec := httptest.NewRecorder()

		assert.Nil(t, a.resolveRequest(rec, withClaims(req)))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("component not found", func(t *testing.T) {
		a := &api{componentStore: &mockComponentStore{
			getFunc: func(ctx context.Context, id uuid.UUID) (*model.Component, error) {
				return nil, fmt.Errorf("component not found: %s", id)
			},
		}}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("component_id", uuid.New().String())
		req.SetPathValue("environment_id", uuid.New().String())
		rec := httptest.NewRecorder()

		assert.Nil(t, a.resolveRequest(rec, withClaims(req)))
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("environment not found", func(t *testing.T) {
		compID, _, appID := testIDs()
		a := &api{
			componentStore: defaultComponentStore(compID, appID),
			environmentStore: &mockEnvironmentStore{
				getFunc: func(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
					return nil, fmt.Errorf("environment not found: %s", id)
				},
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("component_id", compID.String())
		req.SetPathValue("environment_id", uuid.New().String())
		rec := httptest.NewRecorder()

		assert.Nil(t, a.resolveRequest(rec, withClaims(req)))
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("application mismatch", func(t *testing.T) {
		compID := uuid.New()
		envID := uuid.New()
		a := &api{
			componentStore: &mockComponentStore{
				getFunc: func(ctx context.Context, id uuid.UUID) (*model.Component, error) {
					return &model.Component{Id: compID, ApplicationId: uuid.New()}, nil
				},
			},
			environmentStore: &mockEnvironmentStore{
				getFunc: func(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
					return &model.Environment{Id: envID, ApplicationId: uuid.New()}, nil
				},
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("component_id", compID.String())
		req.SetPathValue("environment_id", envID.String())
		rec := httptest.NewRecorder()

		assert.Nil(t, a.resolveRequest(rec, withClaims(req)))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("success", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := &api{
			componentStore:   defaultComponentStore(compID, appID),
			environmentStore: defaultEnvironmentStore(envID, appID),
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("component_id", compID.String())
		req.SetPathValue("environment_id", envID.String())
		rec := httptest.NewRecorder()

		rc := a.resolveRequest(rec, withClaims(req))
		require.NotNil(t, rc)
		assert.Equal(t, compID, rc.componentID)
		assert.Equal(t, envID, rc.environmentID)
		assert.NotNil(t, rc.claims)
	})
}

func TestHandleGetState(t *testing.T) {
	t.Run("no state returns 204", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			getLatestFunc: func(ctx context.Context, cID, eID uuid.UUID) (*model.TerraformState, error) {
				return nil, nil
			},
		}

		rec := httptest.NewRecorder()
		a.handleGetState(rec, stateRequest(http.MethodGet, compID, envID, ""))
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("returns state body", func(t *testing.T) {
		compID, envID, appID := testIDs()
		body := []byte(`{"version":4,"serial":5}`)
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			getLatestFunc: func(ctx context.Context, cID, eID uuid.UUID) (*model.TerraformState, error) {
				return &model.TerraformState{StoragePath: "s/path.tfstate"}, nil
			},
		}
		a.objStore = &mockObjStore{
			getObjectFunc: func(ctx context.Context, bucket, path string) ([]byte, error) {
				return body, nil
			},
		}

		rec := httptest.NewRecorder()
		a.handleGetState(rec, stateRequest(http.MethodGet, compID, envID, ""))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, body, rec.Body.Bytes())
	})

	t.Run("store error returns 500", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			getLatestFunc: func(ctx context.Context, cID, eID uuid.UUID) (*model.TerraformState, error) {
				return nil, fmt.Errorf("db down")
			},
		}

		rec := httptest.NewRecorder()
		a.handleGetState(rec, stateRequest(http.MethodGet, compID, envID, ""))
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandlePostState(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		compID, envID, appID := testIDs()
		stateID := uuid.New()
		stateJSON := `{"version":4,"serial":10,"lineage":"line-1"}`

		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			getLockFunc: func(ctx context.Context, cID, eID uuid.UUID) (*model.TerraformStateLock, error) {
				return nil, nil
			},
			createFunc: func(ctx context.Context, st *model.TerraformState) (*model.TerraformState, error) {
				assert.Equal(t, int64(10), st.Serial)
				assert.Equal(t, "line-1", st.Lineage)
				st.Id = stateID
				return st, nil
			},
			setStoragePathFunc: func(ctx context.Context, id uuid.UUID, path string) error {
				assert.Equal(t, stateID, id)
				assert.Contains(t, path, stateID.String())
				return nil
			},
		}
		a.objStore = &mockObjStore{
			putObjectFunc: func(ctx context.Context, bucket, path string, content []byte) error {
				assert.JSONEq(t, stateJSON, string(content))
				return nil
			},
		}

		rec := httptest.NewRecorder()
		a.handlePostState(rec, stateRequestWithBody(http.MethodPost, compID, envID, stateJSON))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("locked by another returns 423", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			getLockFunc: func(ctx context.Context, cID, eID uuid.UUID) (*model.TerraformStateLock, error) {
				return &model.TerraformStateLock{
					LockId:    "other-lock",
					Who:       "alice",
					CreatedAt: time.Now(),
				}, nil
			},
		}

		req := stateRequestWithBody(http.MethodPost, compID, envID, `{"serial":1,"lineage":"x"}`)
		q := req.URL.Query()
		q.Set("ID", "my-lock")
		req.URL.RawQuery = q.Encode()

		rec := httptest.NewRecorder()
		a.handlePostState(rec, req)
		assert.Equal(t, http.StatusLocked, rec.Code)

		var resp lockJSON
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "other-lock", resp.ID)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)

		rec := httptest.NewRecorder()
		a.handlePostState(rec, stateRequestWithBody(http.MethodPost, compID, envID, "not json"))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestHandleLock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			acquireLockFunc: func(ctx context.Context, lock *model.TerraformStateLock) (*model.TerraformStateLock, error) {
				assert.Equal(t, "lock-abc", lock.LockId)
				return lock, nil
			},
		}

		body := `{"ID":"lock-abc","Operation":"OperationTypePlan","Who":"bob","Version":"1.9.0"}`
		rec := httptest.NewRecorder()
		a.handleLock(rec, stateRequestWithBody("LOCK", compID, envID, body))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("conflict returns 423", func(t *testing.T) {
		compID, envID, appID := testIDs()
		existing := &model.TerraformStateLock{
			LockId: "existing-lock", Who: "alice", CreatedAt: time.Now(),
		}
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			acquireLockFunc: func(ctx context.Context, lock *model.TerraformStateLock) (*model.TerraformStateLock, error) {
				return existing, store.ErrLockConflict
			},
		}

		body := `{"ID":"my-lock","Operation":"OperationTypePlan","Who":"bob"}`
		rec := httptest.NewRecorder()
		a.handleLock(rec, stateRequestWithBody("LOCK", compID, envID, body))
		assert.Equal(t, http.StatusLocked, rec.Code)

		var resp lockJSON
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "existing-lock", resp.ID)
	})

	t.Run("empty lock ID returns 400", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)

		rec := httptest.NewRecorder()
		a.handleLock(rec, stateRequestWithBody("LOCK", compID, envID, `{"ID":""}`))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body returns 400", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)

		rec := httptest.NewRecorder()
		a.handleLock(rec, stateRequestWithBody("LOCK", compID, envID, "not json"))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestHandleUnlock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			releaseLockFunc: func(ctx context.Context, cID, eID uuid.UUID, lockID string) error {
				assert.Equal(t, compID, cID)
				assert.Equal(t, envID, eID)
				assert.Equal(t, "lock-abc", lockID)
				return nil
			},
		}

		rec := httptest.NewRecorder()
		a.handleUnlock(rec, stateRequestWithBody("UNLOCK", compID, envID, `{"ID":"lock-abc"}`))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("conflict returns 423", func(t *testing.T) {
		compID, envID, appID := testIDs()
		a := newTestAPI(compID, envID, appID)
		a.stateStore = &mockStateStore{
			releaseLockFunc: func(ctx context.Context, cID, eID uuid.UUID, lockID string) error {
				return store.ErrLockConflict
			},
			getLockFunc: func(ctx context.Context, cID, eID uuid.UUID) (*model.TerraformStateLock, error) {
				return &model.TerraformStateLock{LockId: "other", Who: "alice", CreatedAt: time.Now()}, nil
			},
		}

		rec := httptest.NewRecorder()
		a.handleUnlock(rec, stateRequestWithBody("UNLOCK", compID, envID, `{"ID":"my-lock"}`))
		assert.Equal(t, http.StatusLocked, rec.Code)
	})
}

func newTestAPI(compID, envID, appID uuid.UUID) *api {
	return &api{
		componentStore:   defaultComponentStore(compID, appID),
		environmentStore: defaultEnvironmentStore(envID, appID),
		stateStore:       &mockStateStore{},
		objStore:         &mockObjStore{},
		objBucket:        "test-bucket",
		logger:           noopLogger(),
	}
}

func stateRequest(method string, compID, envID uuid.UUID, body string) *http.Request {
	return stateRequestWithBody(method, compID, envID, body)
}

func stateRequestWithBody(method string, compID, envID uuid.UUID, body string) *http.Request {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, "/", reader)
	req.SetPathValue("component_id", compID.String())
	req.SetPathValue("environment_id", envID.String())
	return withClaims(req)
}

func noopLogger() *zap.Logger {
	return zap.NewNop()
}
