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
	authnv1 "go.admiral.io/sdk/proto/admiral/api/authentication/v1"
)

const Name = "endpoint.authn"

type api struct {
	provider authn.Provider
	logger   *zap.Logger
	scope    tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	authnService, err := service.GetService[authn.Service]("service.authn")
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
		return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %s", sanitizeOAuthError(req.Error))
	}

	redirectURL, err := a.provider.ValidateStateNonce(ctx, req.State)
	if err != nil {
		return nil, err
	}

	token, err := a.provider.Exchange(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	md := metadata.New(map[string]string{
		"Location":         redirectURL,
		"Set-Access-Token": token.AccessToken,
	})

	if err := grpc.SetHeader(ctx, md); err != nil {
		return nil, err
	}

	return &authnv1.CallbackResponse{}, nil
}

// sanitizeOAuthError maps OAuth error codes to known safe strings.
// Prevents user-controlled error params from being reflected in responses.
func sanitizeOAuthError(errorCode string) string {
	switch errorCode {
	case "access_denied":
		return "access denied by identity provider"
	case "invalid_request":
		return "invalid authentication request"
	case "unauthorized_client":
		return "client not authorized"
	case "unsupported_response_type":
		return "unsupported response type"
	case "invalid_scope":
		return "invalid scope requested"
	case "server_error":
		return "identity provider error"
	case "temporarily_unavailable":
		return "identity provider temporarily unavailable"
	default:
		return "unknown authentication error"
	}
}
