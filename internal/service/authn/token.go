package authn

import "fmt"

type TokenKind string

const (
	TokenKindSession TokenKind = "session"
	TokenKindPAT     TokenKind = "pat"
	TokenKindSAT     TokenKind = "sat"
)

var AllScopes = []string{
	"app:read", "app:write",
	"env:read", "env:write",
	"var:read", "var:write",
	"cluster:read", "cluster:write",
	"connection:read", "connection:write",
	"state:read", "state:write", "state:admin",
	"token:read", "token:write",
	"user:read",
	"runner:exec",
}

var SessionScopes = []string{
	"app:*",
	"env:*",
	"var:*",
	"cluster:*",
	"connection:*",
	"state:*",
	"token:*",
	"user:*",
	"runner:*",
}

var validScopes = func() map[string]struct{} {
	m := make(map[string]struct{}, len(AllScopes))
	for _, s := range AllScopes {
		m[s] = struct{}{}
	}
	return m
}()

func ValidTokenKind(k string) bool {
	switch TokenKind(k) {
	case TokenKindSession, TokenKindPAT, TokenKindSAT:
		return true
	default:
		return false
	}
}

func ValidateScopes(scopes []string) error {
	for _, s := range scopes {
		if _, ok := validScopes[s]; !ok {
			return fmt.Errorf("invalid scope: %q", s)
		}
	}
	return nil
}
