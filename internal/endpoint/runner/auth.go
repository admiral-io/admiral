package runner

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/service/authn"
)

// runnerIDFromClaims extracts the runner UUID from a gRPC context. The
// caller must be authenticated with a runner SAT (subject = runner ID).
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

// authenticateRunner extracts and verifies a runner SAT from an HTTP
// request's Authorization header. Used by the raw-HTTP routes that don't
// flow through the gRPC auth interceptors.
func (a *api) authenticateRunner(r *http.Request) (uuid.UUID, error) {
	token, err := extractBearer(r.Header.Get("Authorization"))
	if err != nil {
		return uuid.Nil, err
	}
	claims, err := a.sessionProvider.Verify(r.Context(), token)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid token")
	}
	if claims.Kind != string(authn.TokenKindSAT) {
		return uuid.Nil, fmt.Errorf("runner SAT required")
	}
	return uuid.Parse(claims.Subject)
}

func extractBearer(header string) (string, error) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", fmt.Errorf("missing or malformed Authorization header")
	}
	if fields[1] == "" {
		return "", fmt.Errorf("empty bearer token")
	}
	return fields[1], nil
}
