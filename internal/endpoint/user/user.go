package user

import (
	"context"
	"time"

	commonv1 "buf.build/gen/go/admiral/common/protocolbuffers/go/admiral/common/v1"
	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/store"
	userv1 "go.admiral.io/sdk/proto/admiral/api/user/v1"
)

const Name = "endpoint.user"

type api struct {
	userStore  *store.UserStore
	tokenStore *store.AuthnTokenStore
	issuer     authn.Issuer
	logger     *zap.Logger
	scope      tally.Scope
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

	tokenStore, err := store.NewAuthnTokenStore(db.GormDB())
	if err != nil {
		return nil, err
	}

	authnService, err := service.GetService[authn.Service]("service.authn")
	if err != nil {
		return nil, err
	}

	return &api{
		userStore:  userStore,
		tokenStore: tokenStore,
		issuer:     authnService,
		logger:     log.Named(Name),
		scope:      scope.SubScope("user"),
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
		User: userToProto(user),
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
		User: userToProto(user),
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

	var expiry *time.Duration
	if req.ExpiresAt != nil {
		d := time.Until(req.ExpiresAt.AsTime())
		if d <= 0 {
			return nil, status.Error(codes.InvalidArgument, "expires_at must be in the future")
		}
		expiry = &d
	}

	token, err := a.issuer.CreateToken(ctx, authn.TokenKindPAT, claims.Subject, scopes, expiry)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create token: %v", err)
	}

	return &userv1.CreatePersonalAccessTokenResponse{
		AccessToken: &commonv1.AccessToken{
			Id:     "", // TODO: parse JTI from token
			Name:   req.GetName(),
			Scopes: scopes,
			Status: commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_ACTIVE,
		},
		PlainTextToken: token.AccessToken,
	}, nil
}

func (a *api) ListPersonalAccessTokens(ctx context.Context, req *userv1.ListPersonalAccessTokensRequest) (*userv1.ListPersonalAccessTokensResponse, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	tokens, err := a.tokenStore.ListBySubject(ctx, claims.Subject, string(model.AuthnTokenKindUser))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list tokens: %v", err)
	}

	resp := &userv1.ListPersonalAccessTokensResponse{}
	for _, t := range tokens {
		resp.AccessTokens = append(resp.AccessTokens, authnTokenToProto(&t))
	}

	return resp, nil
}

func (a *api) GetPersonalAccessToken(ctx context.Context, req *userv1.GetPersonalAccessTokenRequest) (*userv1.GetPersonalAccessTokenResponse, error) {
	id, err := uuid.Parse(req.GetTokenId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid token ID: %v", err)
	}

	token, err := a.tokenStore.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", id)
	}

	return &userv1.GetPersonalAccessTokenResponse{
		AccessToken: authnTokenToProto(token),
	}, nil
}

func (a *api) RevokePersonalAccessToken(ctx context.Context, req *userv1.RevokePersonalAccessTokenRequest) (*userv1.RevokePersonalAccessTokenResponse, error) {
	id, err := uuid.Parse(req.GetTokenId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid token ID: %v", err)
	}

	// Verify the token exists before revoking.
	token, err := a.tokenStore.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", id)
	}

	if err := a.tokenStore.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke token: %v", err)
	}

	token.Status = model.AuthnTokenStatusRevoked
	return &userv1.RevokePersonalAccessTokenResponse{
		AccessToken: authnTokenToProto(token),
	}, nil
}

func userToProto(u *model.User) *userv1.User {
	proto := &userv1.User{
		Id:            u.Id.String(),
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		CreatedAt:     timestamppb.New(u.CreatedAt),
		UpdatedAt:     timestamppb.New(u.UpdatedAt),
	}

	if u.Name != "" {
		proto.DisplayName = &u.Name
	}

	if u.GivenName != "" {
		proto.GivenName = &u.GivenName
	}

	if u.FamilyName != "" {
		proto.FamilyName = &u.FamilyName
	}

	if u.PictureUrl != "" {
		proto.AvatarUrl = &u.PictureUrl
	}

	return proto
}

func authnTokenToProto(t *model.AuthnToken) *commonv1.AccessToken {
	proto := &commonv1.AccessToken{
		Id:        t.Id.String(),
		CreatedAt: timestamppb.New(t.CreatedAt),
		ExpiresAt: timestamppb.New(t.ExpiresAt),
	}

	switch t.Status {
	case model.AuthnTokenStatusActive:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_ACTIVE
	case model.AuthnTokenStatusRevoked:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_REVOKED
	case model.AuthnTokenStatusRotating:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_ROTATING
	}

	return proto
}
