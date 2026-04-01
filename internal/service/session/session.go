package session

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/gormstore"
	"github.com/alexedwards/scs/v2"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/database"
)

const Name = "service.session"

// Status represents the state of a session, independent of the underlying library.
type Status int

const (
	StatusUnmodified Status = iota
	StatusModified
	StatusDestroyed
)

// Service defines the session operations available to the rest of the application.
type Service interface {
	// Data access.
	Get(ctx context.Context, key string) any
	Put(ctx context.Context, key string, val any)
	Remove(ctx context.Context, key string)
	Exists(ctx context.Context, key string) bool
	Clear(ctx context.Context) error

	// Lifecycle.
	Destroy(ctx context.Context) error
	RenewToken(ctx context.Context) error
	Status(ctx context.Context) Status
	Token(ctx context.Context) string

	// HTTP integration.
	HTTPMiddleware() func(http.Handler) http.Handler
	Commit(ctx context.Context) (token string, expiry time.Time, err error)
	Load(ctx context.Context, token string) (context.Context, error)
	GetString(ctx context.Context, key string) string
}

// Get is a generic helper for typed session value retrieval.
func Get[T any](svc Service, ctx context.Context, key string) (T, bool) {
	val := svc.Get(ctx, key)
	if val == nil {
		var zero T
		return zero, false
	}
	typed, ok := val.(T)
	return typed, ok
}

type srv struct {
	sm     *scs.SessionManager
	logger *zap.Logger
}

func New(cfg *config.Config, logger *zap.Logger, _ tally.Scope) (service.Service, error) {
	dbService, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}

	sm := scs.New()
	configure(cfg, sm)

	store, err := gormstore.New(dbService.GormDB())
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}
	sm.Store = store

	return &srv{sm: sm, logger: logger.Named(Name)}, nil
}

func configure(cfg *config.Config, sm *scs.SessionManager) {
	session := cfg.Services.Session
	if session == nil {
		return
	}

	if session.Lifetime > 0 {
		sm.Lifetime = session.Lifetime
	}

	if session.IdleTimeout > 0 {
		sm.IdleTimeout = session.IdleTimeout
	}

	cookie := session.Cookie
	if cookie.Name != "" {
		sm.Cookie.Name = cookie.Name
	}

	if cookie.Domain != "" {
		sm.Cookie.Domain = cookie.Domain
	}

	if cookie.HttpOnly != nil {
		sm.Cookie.HttpOnly = *cookie.HttpOnly
	}

	switch cookie.SameSite {
	case config.SessionSameSiteStrict:
		sm.Cookie.SameSite = http.SameSiteStrictMode
	case config.SessionSameSiteNone:
		sm.Cookie.SameSite = http.SameSiteNoneMode
	default:
		sm.Cookie.SameSite = http.SameSiteLaxMode
	}

	if cookie.Secure != nil {
		sm.Cookie.Secure = *cookie.Secure
	}

	if cookie.Persist != nil {
		sm.Cookie.Persist = *cookie.Persist
	}

	sm.Cookie.Path = "/"
}

func (s *srv) Get(ctx context.Context, key string) any {
	return s.sm.Get(ctx, key)
}

func (s *srv) Put(ctx context.Context, key string, val any) {
	s.sm.Put(ctx, key, val)
}

func (s *srv) Remove(ctx context.Context, key string) {
	s.sm.Remove(ctx, key)
}

func (s *srv) Exists(ctx context.Context, key string) bool {
	return s.sm.Exists(ctx, key)
}

func (s *srv) Clear(ctx context.Context) error {
	return s.sm.Clear(ctx)
}

func (s *srv) Destroy(ctx context.Context) error {
	return s.sm.Destroy(ctx)
}

func (s *srv) RenewToken(ctx context.Context) error {
	return s.sm.RenewToken(ctx)
}

func (s *srv) Status(ctx context.Context) Status {
	switch s.sm.Status(ctx) {
	case scs.Modified:
		return StatusModified
	case scs.Destroyed:
		return StatusDestroyed
	default:
		return StatusUnmodified
	}
}

func (s *srv) Token(ctx context.Context) string {
	return s.sm.Token(ctx)
}

func (s *srv) HTTPMiddleware() func(http.Handler) http.Handler {
	return s.sm.LoadAndSave
}

func (s *srv) Commit(ctx context.Context) (string, time.Time, error) {
	return s.sm.Commit(ctx)
}

func (s *srv) Load(ctx context.Context, token string) (context.Context, error) {
	return s.sm.Load(ctx, token)
}

func (s *srv) GetString(ctx context.Context, key string) string {
	return s.sm.GetString(ctx, key)
}
