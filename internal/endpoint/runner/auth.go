package runner

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/service/authn"
)

// runnerIDFromClaims extracts the runner UUID from a gRPC context. The
// caller must be authenticated with a runner SAT (subject = runner ID).
// Returns gRPC status errors for the unary interceptor path.
func runnerIDFromClaims(ctx context.Context) (uuid.UUID, error) {
	claims, err := authn.ClaimsFromContext(ctx)
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if claims.Kind != string(authn.TokenKindSAT) {
		return uuid.Nil, status.Error(codes.PermissionDenied, "runner SAT required")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, status.Error(codes.Internal, "invalid subject in token")
	}
	return id, nil
}

// runnerIDFromRequest extracts the runner UUID from an HTTP request whose
// context was populated by the httpauth middleware. The middleware
// guarantees claims are present and the kind is SAT; this helper only
// handles parsing the subject as a UUID.
func runnerIDFromRequest(r *http.Request) (uuid.UUID, error) {
	claims, err := authn.ClaimsFromContext(r.Context())
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(claims.Subject)
}
