// Package httpauth provides a configurable bearer/basic-auth middleware for
// raw HTTP routes that don't flow through the gRPC interceptors. Endpoints
// register handlers wrapped in Middleware(cfg); handlers read authenticated
// claims from the request context via authn.ClaimsFromContext.
package httpauth

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"go.admiral.io/admiral/internal/service/authn"
)

// Config configures the HTTP auth middleware. All fields are optional except
// SessionProvider; nil/zero values disable the corresponding check.
type Config struct {
	// SessionProvider verifies tokens against the authn service.
	SessionProvider authn.SessionProvider

	// ScopeForMethod returns the scope a request needs given its HTTP method,
	// or "" to skip the scope check for that method. nil disables scope
	// enforcement entirely.
	ScopeForMethod func(method string) string

	// RequiredKind constrains which token kind is permitted. nil allows any
	// kind. Used by the runner endpoint to reject non-SAT tokens.
	RequiredKind *authn.TokenKind

	// AllowBasicAuth permits credentials via HTTP Basic Auth (password
	// component holds the token). Used by Terraform's HTTP state backend,
	// which sends `username:password` rather than a Bearer header.
	AllowBasicAuth bool
}

// Middleware returns a wrapper that authenticates the incoming request and
// stores the resulting claims in the context via authn.ContextWithClaims.
// Authentication failures short-circuit with 401.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := authenticate(r, cfg)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(authn.ContextWithClaims(r.Context(), claims)))
		})
	}
}

func authenticate(r *http.Request, cfg Config) (*authn.Claims, error) {
	var token string
	if cfg.AllowBasicAuth {
		if _, password, ok := r.BasicAuth(); ok {
			token = password
		}
	}
	if token == "" {
		bearer, err := extractBearer(r.Header.Get("Authorization"))
		if err != nil {
			return nil, fmt.Errorf("authentication required")
		}
		token = bearer
	}

	claims, err := cfg.SessionProvider.Verify(r.Context(), token)
	if err != nil {
		return nil, err
	}

	if cfg.RequiredKind != nil && claims.Kind != string(*cfg.RequiredKind) {
		return nil, fmt.Errorf("token kind not permitted")
	}

	if cfg.ScopeForMethod != nil {
		scope := cfg.ScopeForMethod(r.Method)
		if scope != "" && !slices.Contains(claims.Scopes, scope) {
			return nil, fmt.Errorf("token missing required scope: %s", scope)
		}
	}

	return claims, nil
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
