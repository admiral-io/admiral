package user

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
	userv1 "go.admiral.io/sdk/proto/admiral/user/v1"
)

const Name = "endpoint.user"

type api struct {
	userStore   *store.UserStore
	tokenStore  *store.AccessTokenStore
	tokenIssuer authn.TokenIssuer
	logger      *zap.Logger
	scope       tally.Scope
}

func New(_ *config.Config, log *zap.Logger, scope tally.Scope) (endpoint.Endpoint, error) {
	db, err := service.GetService[database.Service]("service.database")
	if err != nil {
		return nil, err
	}

	userStore, err := store.NewUserStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	tokenStore, err := store.NewAccessTokenStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	authnService, err := service.GetService[authn.Service]("service.authn")
	if err != nil {
		return nil, err
	}

	return &api{
		userStore:   userStore,
		tokenStore:  tokenStore,
		tokenIssuer: authnService,
		logger:      log.Named(Name),
		scope:       scope.SubScope("user"),
	}, nil
}

func (a *api) Register(r endpoint.Registrar) error {
	userv1.RegisterUserAPIServer(r.GRPCServer(), a)
	return r.RegisterJSONGateway(userv1.RegisterUserAPIHandler)
}

func (a *api) GetMe(ctx context.Context, _ *userv1.GetMeRequest) (*userv1.GetMeResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid subject in token: %v", err)
	}

	user, err := a.userStore.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return &userv1.GetMeResponse{
		User: user.ToProto(),
	}, nil
}

func (a *api) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	id, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	user, err := a.userStore.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %s", id)
	}

	return &userv1.GetUserResponse{
		User: user.ToProto(),
	}, nil
}

func (a *api) CreatePersonalAccessToken(ctx context.Context, req *userv1.CreatePersonalAccessTokenRequest) (*userv1.CreatePersonalAccessTokenResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	scopes := req.GetScopes()
	if len(scopes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one scope is required")
	}

	if err := authn.ValidateScopes(scopes); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid scopes: %v", err)
	}

	var expiry *time.Duration
	if req.ExpiresAt != nil {
		d := time.Until(req.ExpiresAt.AsTime())
		if d <= 0 {
			return nil, status.Error(codes.InvalidArgument, "expires_at must be in the future")
		}
		expiry = &d
	}

	accessToken, plaintext, err := a.tokenIssuer.CreateToken(ctx, authn.TokenKindPAT, req.GetName(), claims.Subject, scopes, expiry)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create token: %v", err)
	}

	return &userv1.CreatePersonalAccessTokenResponse{
		AccessToken:    accessToken.ToProto(),
		PlainTextToken: plaintext,
	}, nil
}

func (a *api) ListPersonalAccessTokens(ctx context.Context, req *userv1.ListPersonalAccessTokensRequest) (*userv1.ListPersonalAccessTokensResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	tokens, err := a.tokenStore.ListBySubject(ctx, claims.Subject, string(model.AccessTokenKindPAT))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list tokens: %v", err)
	}

	resp := &userv1.ListPersonalAccessTokensResponse{}
	for _, t := range tokens {
		resp.AccessTokens = append(resp.AccessTokens, t.ToProto())
	}

	return resp, nil
}

func (a *api) GetPersonalAccessToken(ctx context.Context, req *userv1.GetPersonalAccessTokenRequest) (*userv1.GetPersonalAccessTokenResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	token, err := a.tokenStore.Get(ctx, req.GetTokenId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", req.GetTokenId())
	}

	if token.Subject != claims.Subject {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", req.GetTokenId())
	}

	return &userv1.GetPersonalAccessTokenResponse{
		AccessToken: token.ToProto(),
	}, nil
}

func (a *api) UpdatePersonalAccessToken(ctx context.Context, req *userv1.UpdatePersonalAccessTokenRequest) (*userv1.UpdatePersonalAccessTokenResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	token, err := a.tokenStore.Get(ctx, req.GetTokenId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", req.GetTokenId())
	}

	if token.Subject != claims.Subject {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", req.GetTokenId())
	}

	if token.Status != model.AccessTokenStatusActive {
		return nil, status.Error(codes.FailedPrecondition, "only active tokens can be updated")
	}

	updates := make(map[string]any)

	if req.Name != nil {
		updates["name"] = req.GetName()
	}

	if req.Scopes != nil {
		if len(req.GetScopes()) == 0 {
			return nil, status.Error(codes.InvalidArgument, "at least one scope is required")
		}
		if err := authn.ValidateScopes(req.GetScopes()); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid scopes: %v", err)
		}
		updates["scopes"] = req.GetScopes()
	}

	if len(updates) == 0 {
		return &userv1.UpdatePersonalAccessTokenResponse{
			AccessToken: token.ToProto(),
		}, nil
	}

	if err := a.tokenStore.Update(ctx, req.GetTokenId(), updates); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update token: %v", err)
	}

	updated, err := a.tokenStore.Get(ctx, req.GetTokenId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve updated token: %v", err)
	}

	return &userv1.UpdatePersonalAccessTokenResponse{
		AccessToken: updated.ToProto(),
	}, nil
}

func (a *api) RevokePersonalAccessToken(ctx context.Context, req *userv1.RevokePersonalAccessTokenRequest) (*userv1.RevokePersonalAccessTokenResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	token, err := a.tokenStore.Get(ctx, req.GetTokenId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", req.GetTokenId())
	}

	if token.Subject != claims.Subject {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", req.GetTokenId())
	}

	if err := a.tokenStore.Delete(ctx, req.GetTokenId()); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke token: %v", err)
	}

	token.Status = model.AccessTokenStatusRevoked
	return &userv1.RevokePersonalAccessTokenResponse{
		AccessToken: token.ToProto(),
	}, nil
}
