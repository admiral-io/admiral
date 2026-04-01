package authz

import (
	"context"
	"slices"
	"strings"

	commonv1 "buf.build/gen/go/admiral/common/protocolbuffers/go/admiral/common/v1"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/gateway/meta"
	"go.admiral.io/admiral/internal/middleware"
	"go.admiral.io/admiral/internal/service/authn"
)

const Name = "middleware.authz"

type mid struct {
	logger *zap.Logger
}

func New(_ *config.Config, logger *zap.Logger, _ tally.Scope) (middleware.Middleware, error) {
	return &mid{
		logger: logger.Named(Name),
	}, nil
}

func (m *mid) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		rule := getAuthRule(info.FullMethod)
		if rule == nil {
			// No AuthRule annotation — method does not require authorization.
			return handler(ctx, req)
		}

		claims, err := authn.ClaimsFromContext(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "authorization requires authentication")
		}

		if err := checkTokenType(rule, claims); err != nil {
			m.logger.Warn("authz denied: token type",
				zap.String("method", info.FullMethod),
				zap.String("kind", claims.Kind),
				zap.Strings("allowed", rule.GetAllowedTokenTypes()),
			)
			return nil, err
		}

		if err := checkScope(rule, claims); err != nil {
			m.logger.Warn("authz denied: scope",
				zap.String("method", info.FullMethod),
				zap.String("required", rule.GetScope()),
				zap.Strings("token_scopes", claims.Scopes),
			)
			return nil, err
		}

		return handler(ctx, req)
	}
}

func getAuthRule(fullMethod string) *commonv1.AuthRule {
	opts := meta.GetMethodOptions(fullMethod)
	if opts == nil {
		return nil
	}

	if !proto.HasExtension(opts, commonv1.E_Authz) {
		return nil
	}

	rule, ok := proto.GetExtension(opts, commonv1.E_Authz).(*commonv1.AuthRule)
	if !ok {
		return nil
	}

	return rule
}

func checkTokenType(rule *commonv1.AuthRule, claims *authn.Claims) error {
	allowed := rule.GetAllowedTokenTypes()
	if len(allowed) == 0 {
		return nil
	}

	if !slices.Contains(allowed, claims.Kind) {
		return status.Errorf(codes.PermissionDenied,
			"token type %q is not allowed for this endpoint", claims.Kind)
	}

	return nil
}

func checkScope(rule *commonv1.AuthRule, claims *authn.Claims) error {
	required := rule.GetScope()
	if required == "" {
		return nil
	}

	if hasScope(claims.Scopes, required) {
		return nil
	}

	return status.Errorf(codes.PermissionDenied, "missing required scope %q", required)
}

func hasScope(granted []string, required string) bool {
	for _, s := range granted {
		if s == required {
			return true
		}

		// Check resource wildcard: "app:*" matches "app:read"
		if len(s) > 2 && s[len(s)-1] == '*' && s[len(s)-2] == ':' {
			prefix := s[:len(s)-1] // "app:"
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}

	return false
}
