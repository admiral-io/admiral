package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.admiral.io/admiral/internal/config"
)

// newTestService creates a session service backed by SCS's default in-memory store.
func newTestService() *srv {
	sm := scs.New()
	return &srv{sm: sm}
}

// withSessionContext runs fn inside a loaded session context using httptest.
func withSessionContext(t *testing.T, s *srv, fn func(ctx context.Context)) {
	t.Helper()

	handler := s.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fn(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestPutGet(t *testing.T) {
	s := newTestService()

	withSessionContext(t, s, func(ctx context.Context) {
		s.Put(ctx, "username", "alice")
		got := s.Get(ctx, "username")
		assert.Equal(t, "alice", got)
	})
}

func TestGetMissing(t *testing.T) {
	s := newTestService()

	withSessionContext(t, s, func(ctx context.Context) {
		got := s.Get(ctx, "nonexistent")
		assert.Nil(t, got)
	})
}

func TestRemoveExists(t *testing.T) {
	s := newTestService()

	withSessionContext(t, s, func(ctx context.Context) {
		s.Put(ctx, "key", "value")
		assert.True(t, s.Exists(ctx, "key"))

		s.Remove(ctx, "key")
		assert.False(t, s.Exists(ctx, "key"))
		assert.Nil(t, s.Get(ctx, "key"))
	})
}

func TestClear(t *testing.T) {
	s := newTestService()

	withSessionContext(t, s, func(ctx context.Context) {
		s.Put(ctx, "a", 1)
		s.Put(ctx, "b", 2)

		err := s.Clear(ctx)
		require.NoError(t, err)

		assert.False(t, s.Exists(ctx, "a"))
		assert.False(t, s.Exists(ctx, "b"))
	})
}

func TestStatus(t *testing.T) {
	s := newTestService()

	withSessionContext(t, s, func(ctx context.Context) {
		assert.Equal(t, StatusUnmodified, s.Status(ctx))

		s.Put(ctx, "key", "value")
		assert.Equal(t, StatusModified, s.Status(ctx))
	})
}

func TestStatusDestroyed(t *testing.T) {
	s := newTestService()

	withSessionContext(t, s, func(ctx context.Context) {
		err := s.Destroy(ctx)
		require.NoError(t, err)
		assert.Equal(t, StatusDestroyed, s.Status(ctx))
	})
}

func TestHTTPMiddleware(t *testing.T) {
	s := newTestService()

	mw := s.HTTPMiddleware()
	require.NotNil(t, mw)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetGenericHelper(t *testing.T) {
	s := newTestService()

	withSessionContext(t, s, func(ctx context.Context) {
		s.Put(ctx, "count", 42)
		s.Put(ctx, "name", "bob")

		count, ok := Get[int](s, ctx, "count")
		assert.True(t, ok)
		assert.Equal(t, 42, count)

		name, ok := Get[string](s, ctx, "name")
		assert.True(t, ok)
		assert.Equal(t, "bob", name)

		// Wrong type returns zero value and false.
		_, ok = Get[int](s, ctx, "name")
		assert.False(t, ok)

		// Missing key returns zero value and false.
		val, ok := Get[string](s, ctx, "missing")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})
}

func TestConfigureSession(t *testing.T) {
	t.Run("nil session config", func(t *testing.T) {
		sm := scs.New()
		configure(&config.Config{}, sm)
		// Should use SCS defaults, not panic.
		assert.Equal(t, 24*time.Hour, sm.Lifetime)
	})

	t.Run("custom values applied", func(t *testing.T) {
		httpOnly := false
		secure := false
		persist := true

		sm := scs.New()
		cfg := &config.Config{
			Services: config.Services{
				Session: &config.Session{
					Lifetime:    2 * time.Hour,
					IdleTimeout: 30 * time.Minute,
					Cookie: config.Cookie{
						Name:     "test_session",
						Domain:   "example.com",
						HttpOnly: &httpOnly,
						SameSite: config.SessionSameSiteStrict,
						Secure:   &secure,
						Persist:  &persist,
					},
				},
			},
		}
		configure(cfg, sm)

		assert.Equal(t, 2*time.Hour, sm.Lifetime)
		assert.Equal(t, 30*time.Minute, sm.IdleTimeout)
		assert.Equal(t, "test_session", sm.Cookie.Name)
		assert.Equal(t, "example.com", sm.Cookie.Domain)
		assert.Equal(t, "/", sm.Cookie.Path)
		assert.False(t, sm.Cookie.HttpOnly)
		assert.False(t, sm.Cookie.Secure)
		assert.True(t, sm.Cookie.Persist)
		assert.Equal(t, http.SameSiteStrictMode, sm.Cookie.SameSite)
	})

	t.Run("SameSite none", func(t *testing.T) {
		sm := scs.New()
		cfg := &config.Config{
			Services: config.Services{
				Session: &config.Session{
					Cookie: config.Cookie{SameSite: config.SessionSameSiteNone},
				},
			},
		}
		configure(cfg, sm)
		assert.Equal(t, http.SameSiteNoneMode, sm.Cookie.SameSite)
	})

	t.Run("SameSite defaults to lax", func(t *testing.T) {
		sm := scs.New()
		cfg := &config.Config{
			Services: config.Services{
				Session: &config.Session{},
			},
		}
		configure(cfg, sm)
		assert.Equal(t, http.SameSiteLaxMode, sm.Cookie.SameSite)
	})
}
