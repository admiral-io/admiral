package authn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonv1 "buf.build/gen/go/admiral/common/protocolbuffers/go/admiral/common/v1"
	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/gateway/meta"
	"go.admiral.io/admiral/internal/gateway/mux"
	"go.admiral.io/admiral/internal/middleware"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/session"
)

const Name = "middleware.authn"

type mid struct {
	provider          authn.Provider
	issuer            authn.Issuer
	session           session.Service
	sessionRefreshTTL time.Duration
	logger            *zap.Logger
}

func New(cfg *config.Config, logger *zap.Logger, _ tally.Scope) (middleware.Middleware, error) {
	authnService, err := service.GetService[authn.Service]("service.authn")
	if err != nil {
		return nil, fmt.Errorf("failed to get authn service: %w", err)
	}

	sessionService, err := service.GetService[session.Service]("service.session")
	if err != nil {
		return nil, fmt.Errorf("failed to get session service: %w", err)
	}

	return &mid{
		provider:          authnService,
		issuer:            authnService,
		session:           sessionService,
		sessionRefreshTTL: cfg.Services.Authn.SessionRefreshTTL,
		logger:            logger.Named(Name),
	}, nil
}

func (m *mid) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		authenticatedCtx, authErr := m.authenticate(ctx)

		if !requiresAuth(info.FullMethod) {
			// No AuthRule annotation — auth is optional.
			if authErr != nil {
				return handler(authn.ContextWithAnonymousClaims(ctx), req)
			}
			return handler(authenticatedCtx, req)
		}

		// Has AuthRule annotation — auth is required.
		if authErr != nil {
			return nil, status.Error(codes.Unauthenticated, authErr.Error())
		}
		return handler(authenticatedCtx, req)
	}
}

func requiresAuth(fullMethod string) bool {
	opts := meta.GetMethodOptions(fullMethod)
	if opts == nil {
		return false
	}
	return proto.HasExtension(opts, commonv1.E_Authz)
}

func (m *mid) authenticate(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no headers present on request")
	}

	if tokens := md.Get("authorization"); len(tokens) > 0 {
		token, err := parseBearerToken(tokens[0])
		if err != nil {
			return nil, err
		}

		claims, err := m.provider.Verify(ctx, token)
		if err != nil {
			return nil, err
		}

		return authn.ContextWithClaims(ctx, claims), nil
	}

	return m.authenticateSession(ctx, md)
}

func (m *mid) authenticateSession(ctx context.Context, md metadata.MD) (context.Context, error) {
	cookies := md.Get("grpcgateway-cookie")
	if len(cookies) == 0 {
		return nil, errors.New("no session cookie present")
	}

	sid, err := mux.GetCookieValue(cookies, "session")
	if err != nil {
		return nil, errors.New("failed to extract session cookie")
	}

	sessionCtx, err := m.session.Load(ctx, sid)
	if err != nil {
		return nil, err
	}

	rawToken := m.session.GetString(sessionCtx, "accessToken")
	if rawToken == "" {
		return nil, errors.New("no access token in session")
	}

	claims, err := m.provider.Verify(ctx, rawToken)
	if err == nil {
		return authn.ContextWithClaims(ctx, claims), nil
	}

	m.logger.Debug("token verification failed, attempting session refresh", zap.Error(err))
	return m.refreshAndVerify(ctx, sessionCtx, rawToken)
}

func (m *mid) refreshAndVerify(ctx context.Context, sessionCtx context.Context, expiredToken string) (context.Context, error) {
	// Parse the expired token to get the JTI (without validating expiry).
	expiredClaims, err := authn.ParseTokenWithoutValidation(expiredToken)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expired token: %w", err)
	}

	tokenID, err := uuid.Parse(expiredClaims.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ID: %w", err)
	}

	// Enforce the absolute session window before attempting refresh.
	sessionCreatedAt := m.getSessionCreatedAt(sessionCtx)
	if sessionCreatedAt.IsZero() {
		return nil, errors.New("session has no creation time, re-authentication required")
	}

	if time.Since(sessionCreatedAt) > m.sessionRefreshTTL {
		return nil, fmt.Errorf("session has exceeded maximum lifetime (%s), re-authentication required", m.sessionRefreshTTL)
	}

	newToken, err := m.issuer.RefreshToken(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("session refresh failed: %w", err)
	}

	m.session.Put(sessionCtx, "accessToken", newToken.AccessToken)

	claims, err := m.provider.Verify(ctx, newToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify refreshed token: %w", err)
	}

	return authn.ContextWithClaims(ctx, claims), nil
}

func (m *mid) getSessionCreatedAt(sessionCtx context.Context) time.Time {
	createdAt, ok := session.Get[int64](m.session, sessionCtx, "sessionCreatedAt")
	if !ok || createdAt == 0 {
		return time.Time{}
	}

	return time.Unix(createdAt, 0)
}

func parseBearerToken(authHeader string) (string, error) {
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", errors.New("bad token format, expected Authorization: Bearer <token>")
	}

	token := fields[1]
	if token == "" {
		return "", errors.New("token not present in authorization header")
	}

	return token, nil
}
