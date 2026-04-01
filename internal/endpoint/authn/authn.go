package authn

import (
	"context"
	"fmt"

	"github.com/uber-go/tally/v4"
	"go.admiral.io/admiral/internal/gateway/mux"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/gateway/log"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/session"
	authnv1 "go.admiral.io/sdk/proto/admiral/api/authentication/v1"
)

const Name = "endpoint.authn"

type api struct {
	provider authn.Provider
	issuer   authn.Issuer
	session  session.Service
	logger   *zap.Logger
	scope    tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	authnService, err := service.GetService[authn.Service]("service.authn")
	if err != nil {
		return nil, err
	}

	sessionService, err := service.GetService[session.Service]("service.session")
	if err != nil {
		return nil, fmt.Errorf("failed to get session service: %w", err)
	}

	return &api{
		provider: authnService,
		issuer:   authnService,
		session:  sessionService,
		logger:   log.Named("authn"),
		scope:    scope.SubScope("authn"),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	authnv1.RegisterAuthenticationAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(authnv1.RegisterAuthenticationAPIHandler)
}

func (a *api) Login(ctx context.Context, req *authnv1.LoginRequest) (*authnv1.LoginResponse, error) {
	resp, err := a.loginViaRefresh(ctx, req.RedirectUrl)
	if err != nil {
		a.logger.Info("login via refresh token failed, continuing regular auth flow", log.ErrorField(err))
	} else if resp != nil {
		return resp, nil
	}

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

	if token.RefreshToken != "" {
		md.Set("Set-Refresh-Token", token.RefreshToken)
	}

	if err := grpc.SetHeader(ctx, md); err != nil {
		return nil, err
	}

	return &authnv1.CallbackResponse{}, nil
}

func (a *api) loginViaRefresh(ctx context.Context, redirectURL string) (*authnv1.LoginResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil
	}

	cookies := md.Get("grpcgateway-cookie")
	if len(cookies) == 0 {
		return nil, nil
	}

	sid, err := mux.GetCookieValue(cookies, "session")
	if err != nil {
		return nil, nil
	}

	sessionCtx, err := a.session.Load(ctx, sid)
	if err != nil {
		return nil, nil
	}

	refreshToken := a.session.GetString(sessionCtx, "refreshToken")
	if refreshToken == "" {
		return nil, nil
	}

	newToken, err := a.issuer.RefreshToken(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})
	if err != nil {
		return nil, err
	}

	err = grpc.SetHeader(ctx, metadata.New(map[string]string{
		"Location":          redirectURL,
		"Set-Access-Token":  newToken.AccessToken,
		"Set-Refresh-Token": newToken.RefreshToken,
	}))
	if err != nil {
		return nil, err
	}

	return &authnv1.LoginResponse{
		Return: &authnv1.LoginResponse_Token_{
			Token: &authnv1.LoginResponse_Token{
				AccessToken:  newToken.AccessToken,
				RefreshToken: newToken.RefreshToken,
			},
		},
	}, nil
}
