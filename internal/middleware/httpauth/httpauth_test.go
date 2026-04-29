package httpauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.admiral.io/admiral/internal/service/authn"
)

type mockSessionProvider struct {
	verifyFunc func(ctx context.Context, credential string) (*authn.Claims, error)
}

func (m *mockSessionProvider) Verify(ctx context.Context, credential string) (*authn.Claims, error) {
	return m.verifyFunc(ctx, credential)
}

func (m *mockSessionProvider) RefreshSession(ctx context.Context, sessionToken string) error {
	return nil
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

func TestMiddleware(t *testing.T) {
	stateScope := func(method string) string {
		if method == http.MethodGet {
			return "state:read"
		}
		return "state:write"
	}
	sat := authn.TokenKindSAT

	t.Run("no auth header", func(t *testing.T) {
		cfg := Config{
			SessionProvider: &mockSessionProvider{
				verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
					t.Fatal("verify should not be called")
					return nil, nil
				},
			},
		}
		handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		cfg := Config{
			SessionProvider: &mockSessionProvider{
				verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
					return nil, fmt.Errorf("invalid token")
				},
			},
		}
		handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bad")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing scope", func(t *testing.T) {
		cfg := Config{
			SessionProvider: &mockSessionProvider{
				verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
					return &authn.Claims{Subject: "u", Scopes: []string{"runner:exec"}}, nil
				},
			},
			ScopeForMethod: stateScope,
		}
		handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer valid")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("wrong kind", func(t *testing.T) {
		cfg := Config{
			SessionProvider: &mockSessionProvider{
				verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
					return &authn.Claims{Subject: "u", Kind: string(authn.TokenKindPAT)}, nil
				},
			},
			RequiredKind: &sat,
		}
		handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer pat")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("bearer success populates claims context", func(t *testing.T) {
		cfg := Config{
			SessionProvider: &mockSessionProvider{
				verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
					assert.Equal(t, "adms_tok", cred)
					return &authn.Claims{Subject: "u", Scopes: []string{"state:read"}}, nil
				},
			},
			ScopeForMethod: stateScope,
		}
		var called bool
		handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			claims, err := authn.ClaimsFromContext(r.Context())
			require.NoError(t, err)
			assert.Equal(t, "u", claims.Subject)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer adms_tok")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.True(t, called)
	})

	t.Run("basic auth success when allowed", func(t *testing.T) {
		cfg := Config{
			SessionProvider: &mockSessionProvider{
				verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
					assert.Equal(t, "adms_tok", cred)
					return &authn.Claims{Subject: "u", Scopes: []string{"state:read"}}, nil
				},
			},
			ScopeForMethod: stateScope,
			AllowBasicAuth: true,
		}
		var called bool
		handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("", "adms_tok")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.True(t, called)
	})

	t.Run("basic auth rejected when not allowed", func(t *testing.T) {
		cfg := Config{
			SessionProvider: &mockSessionProvider{
				verifyFunc: func(ctx context.Context, cred string) (*authn.Claims, error) {
					t.Fatal("verify should not be called")
					return nil, nil
				},
			},
		}
		handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("", "adms_tok")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
