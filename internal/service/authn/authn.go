package authn

import (
	"context"
	"time"

	"github.com/uber-go/tally/v4"
	"go.admiral.io/admiral/internal/service/database"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/service"
)

const Name = "service.authn"

var AlwaysAllowedMethods = []string{
	"/admiral.api.authentication.v1.AuthenticationAPI/Callback",
	"/admiral.api.authentication.v1.AuthenticationAPI/Login",
	"/admiral.api.healthcheck.v1.HealthcheckAPI/*",
}

type Service interface {
	Issuer
	Provider
}

type Provider interface {
	GetStateNonce(ctx context.Context, redirectURL string) (string, error)
	ValidateStateNonce(ctx context.Context, state string) (string, error)
	GetAuthCodeURL(ctx context.Context, state string) (string, error)
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	Verify(ctx context.Context, raw string) (*Claims, error)
}

type Issuer interface {
	CreateToken(ctx context.Context, subject string, expiry *time.Duration) (*oauth2.Token, error)
	RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error)
	RevokeToken(ctx context.Context, token *oauth2.Token) error
}

func New(cfg *config.Config, logger *zap.Logger, scope tally.Scope) (service.Service, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	store, err := newStore(cfg, db.GormDB())
	if err != nil {
		return nil, err
	}

	return NewOIDCProvider(cfg, logger, store)
}
