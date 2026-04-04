package authn

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	Subject string
	Kind    string
	Scopes  []string
}

func (c Claims) Validate() error {
	var missing []string

	if _, err := uuid.Parse(c.Subject); err != nil {
		missing = append(missing, "subject (valid UUID required)")
	}

	if c.Kind != "" && !ValidTokenKind(c.Kind) {
		missing = append(missing, fmt.Sprintf("kind (got %q, must be one of: session, pat, sat)", c.Kind))
	}

	if len(missing) > 0 {
		return fmt.Errorf("validation failed: invalid claims: %s", strings.Join(missing, ", "))
	}

	return nil
}

// stateClaims are JWT claims for OIDC state nonces (CSRF protection in login flow).
type stateClaims struct {
	*jwt.RegisteredClaims
	RedirectURL string `json:"redirect"`
}

func (c *stateClaims) Validate() error {
	if c.RegisteredClaims == nil {
		return errors.New("validation failed: registered claims are required")
	}

	if strings.TrimSpace(c.RedirectURL) == "" {
		return errors.New("validation failed: redirect URL claim is required")
	}

	if c.ExpiresAt != nil && !c.ExpiresAt.After(time.Now()) {
		return errors.New("validation failed: token has expired")
	}

	if c.IssuedAt != nil && c.IssuedAt.After(time.Now()) {
		return errors.New("validation failed: token issued in the future")
	}

	return nil
}
