package authn

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
)

const Name = "service.authn"

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
	CreateToken(ctx context.Context, kind TokenKind, name string, subject string, scopes []string, expiry *time.Duration) (*model.AuthnToken, *oauth2.Token, error)
	RefreshToken(ctx context.Context, tokenID uuid.UUID) (*oauth2.Token, error)
	RevokeToken(ctx context.Context, token *oauth2.Token) error
	RevokeAllTokens(ctx context.Context, subject string) (int64, error)
}

func New(cfg *config.Config, logger *zap.Logger, _ tally.Scope) (service.Service, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	tokens, err := store.NewAuthnTokenStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	users, err := store.NewUserStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	return NewOIDCProvider(cfg, logger, tokens, users)
}
