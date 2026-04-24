package authn

import (
	"context"
	"time"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
)

const Name = "service.authn"

type Service interface {
	SessionProvider
	TokenIssuer
	Provider
}

// Provider handles the OIDC login flow (state nonces, auth code exchange).
type Provider interface {
	GetStateNonce(ctx context.Context, redirectURL string) (string, error)
	ValidateStateNonce(ctx context.Context, state string) (string, error)
	GetAuthCodeURL(ctx context.Context, state string) (string, error)
	Exchange(ctx context.Context, code string) (sessionToken string, err error)
}

// SessionProvider is the single auth verification entry point.
// Verify accepts any credential type and dispatches internally:
//   - "admp_..." → personal access token
//   - "adms_..." → service access token
//   - "adme_..." → session token
type SessionProvider interface {
	Verify(ctx context.Context, credential string) (*Claims, error)
	RefreshSession(ctx context.Context, sessionToken string) error
}

// TokenIssuer handles access token (PAT/SAT) CRUD.
type TokenIssuer interface {
	CreateToken(ctx context.Context, kind TokenKind, binding model.AccessTokenBindingType, name string, subject string, scopes []string, expiry *time.Duration) (*model.AccessToken, string, error)
	RevokeToken(ctx context.Context, rawToken string) error
	RevokeAllTokens(ctx context.Context, subject string) (int64, error)
}

func New(cfg *config.Config, logger *zap.Logger, _ tally.Scope) (service.Service, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	tokens, err := store.NewTokenStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	users, err := store.NewUserStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	return NewOIDCProvider(cfg, logger, tokens, users)
}
