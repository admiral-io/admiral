package authn

import (
	"context"
	"fmt"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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
		logger:   log.Named("authn"),
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
		Return: &authnv1.LoginResponse_AuthUrl{AuthUrl: authURL},
	}, nil
}

func (a *api) Callback(ctx context.Context, req *authnv1.CallbackRequest) (*authnv1.CallbackResponse, error) {
	if req.Error != "" {
		return nil, fmt.Errorf("%s: %s", req.Error, req.ErrorDescription)
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
