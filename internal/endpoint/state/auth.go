package state

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"go.admiral.io/admiral/internal/service/authn"
)

const stateScope = "state:rw"

// claimsKey is the context key for authenticated claims.
type claimsKey struct{}

// withAuth is an HTTP middleware that extracts Basic Auth or Bearer token,
// verifies it, checks scope, and injects claims into the request context.
func (a *api) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := a.extractAndVerify(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey{}, claims)
		next(w, r.WithContext(ctx))
	}
}

// extractAndVerify extracts credentials from Basic Auth or Bearer header,
// verifies the token, and checks for the required state scope.
func (a *api) extractAndVerify(r *http.Request) (*authn.Claims, error) {
	_, password, ok := r.BasicAuth()
	var claims *authn.Claims
	var err error
	if !ok {
		token, tokenErr := extractBearer(r.Header.Get("Authorization"))
		if tokenErr != nil {
			return nil, fmt.Errorf("authentication required")
		}
		claims, err = a.sessionProvider.Verify(r.Context(), token)
	} else {
		claims, err = a.sessionProvider.Verify(r.Context(), password)
	}
	if err != nil {
		return nil, err
	}

	if !slices.Contains(claims.Scopes, stateScope) {
		return nil, fmt.Errorf("token missing required scope: %s", stateScope)
	}

	return claims, nil
}

// claimsFromContext retrieves the authenticated claims set by withAuth.
func claimsFromContext(ctx context.Context) *authn.Claims {
	claims, _ := ctx.Value(claimsKey{}).(*authn.Claims)
	return claims
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