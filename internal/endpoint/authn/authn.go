package authn

import (
	"context"
	"fmt"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	authnv1 "go.admiral.io/sdk/proto/admiral/authentication/v1"
)

const Name = "endpoint.authn"

type api struct {
	provider authn.Provider
	logger   *zap.Logger
	scope    tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	authnService, err := service.GetService[authn.Service](authn.Name)
	if err != nil {
		return nil, err
	}

	return &api{
		provider: authnService,
		logger:   log.Named(Name),
		scope:    scope.SubScope("authn"),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	authnv1.RegisterAuthenticationAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(authnv1.RegisterAuthenticationAPIHandler)
}

func (a *api) Login(ctx context.Context, req *authnv1.LoginRequest) (*authnv1.LoginResponse, error) {
	state, err := a.provider.GetStateNonce(ctx, req.RedirectUrl)
	if err != nil {
		return nil, err
	}

	authURL, err := a.provider.GetAuthCodeURL(ctx, state)
	if err != nil {
		return nil, err
	}

	if err := grpc.SetHeader(ctx, metadata.Pairs("Location", authURL)); err != nil {
		return nil, err
	}

	return &authnv1.LoginResponse{
		AuthUrl: authURL,
	}, nil
}

func (a *api) Callback(ctx context.Context, req *authnv1.CallbackRequest) (*authnv1.CallbackResponse, error) {
	if req.Error != "" {
		a.logger.Warn("OAuth callback error",
			zap.String("error", req.Error),
			zap.String("error_description", req.ErrorDescription),
		)

		var safeError string
		switch req.Error {
		case "access_denied":
			safeError = "access denied by identity provider"
		case "invalid_request":
			safeError = "invalid authentication request"
		case "unauthorized_client":
			safeError = "client not authorized"
		case "unsupported_response_type":
			safeError = "unsupported response type"
		case "invalid_scope":
			safeError = "invalid scope requested"
		case "server_error":
			safeError = "identity provider error"
		case "temporarily_unavailable":
			safeError = "identity provider temporarily unavailable"
		default:
			safeError = "unknown authentication error"
		}

		return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %s", safeError)
	}

	redirectURL, err := a.provider.ValidateStateNonce(ctx, req.State)
	if err != nil {
		return nil, err
	}

	sessionID, err := a.provider.Exchange(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	md := metadata.New(map[string]string{
		"Location":          redirectURL,
		"Set-Session-Token": sessionID,
	})

	if err := grpc.SetHeader(ctx, md); err != nil {
		return nil, err
	}

	return &authnv1.CallbackResponse{}, nil
}
