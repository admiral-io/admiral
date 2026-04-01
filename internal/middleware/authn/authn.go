package authn

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/gateway/mux"
	"go.admiral.io/admiral/internal/middleware"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/session"
)

const Name = "middleware.authn"

type mid struct {
	provider authn.Provider
	session  session.Service
}

func New(_ *config.Config, _ *zap.Logger, _ tally.Scope) (middleware.Middleware, error) {
	authnService, err := service.GetService[authn.Service]("service.authn")
	if err != nil {
		return nil, fmt.Errorf("failed to get authn service: %w", err)
	}

	sessionService, err := service.GetService[session.Service]("service.session")
	if err != nil {
		return nil, fmt.Errorf("failed to get session service: %w", err)
	}

	return &mid{
		provider: authnService,
		session:  sessionService,
	}, nil
}

func (m *mid) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authenticatedCtx, authErr := m.authenticate(ctx)

		checkRequired := true
		for _, allow := range authn.AlwaysAllowedMethods {
			if middleware.MatchMethodOrResource(allow, info.FullMethod) {
				checkRequired = false
				break
			}
		}

		if checkRequired {
			if authErr != nil {
				return nil, status.New(codes.Unauthenticated, authErr.Error()).Err()
			}
			return handler(authenticatedCtx, req)
		}

		if _, err := authn.ClaimsFromContext(authenticatedCtx); err != nil {
			return handler(authn.ContextWithAnonymousClaims(ctx), req)
		}
		return handler(authenticatedCtx, req)
	}
}

func (m *mid) authenticate(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no headers present on request")
	}

	token, err := m.getToken(ctx, md)
	if err != nil {
		return nil, err
	}

	claims, err := m.provider.Verify(ctx, token)
	if err != nil {
		return nil, err
	}

	return authn.ContextWithClaims(ctx, claims), nil
}

func (m *mid) getToken(ctx context.Context, md metadata.MD) (string, error) {
	if tokens := md.Get("authorization"); len(tokens) > 0 {
		authHeader := tokens[0]
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

	cookies := md.Get("grpcgateway-cookie")
	if len(cookies) == 0 {
		return "", errors.New("token not present in authorization header or cookies")
	}

	sid, err := mux.GetCookieValue(cookies, "session")
	if err != nil {
		return "", errors.New("failed to extract session cookie")
	}

	sessionCtx, err := m.session.Load(ctx, sid)
	if err != nil {
		return "", err
	}

	return m.session.GetString(sessionCtx, "accessToken"), nil
}
