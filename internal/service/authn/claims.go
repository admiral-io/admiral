package authn

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

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

type Claims struct {
	*jwt.RegisteredClaims
	ExternalSubject string   `json:"external_sub,omitempty"`
	Kind            string   `json:"kind,omitempty"`
	Email           string   `json:"email,omitempty"`
	EmailVerified   bool     `json:"email_verified,omitempty"`
	Name            string   `json:"name,omitempty"`
	GivenName       string   `json:"given_name,omitempty"`
	FamilyName      string   `json:"family_name,omitempty"`
	Picture         string   `json:"picture,omitempty"`
	Groups          []string `json:"groups,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
}

func (c Claims) Validate() error {
	if c.RegisteredClaims == nil {
		return errors.New("validation failed: registered claims are required")
	}

	var missing []string

	if _, err := uuid.Parse(c.Subject); err != nil {
		missing = append(missing, "subject (valid UUID required)")
	}

	if c.Kind != "" && !ValidTokenKind(c.Kind) {
		missing = append(missing, fmt.Sprintf("kind (got %q, must be one of: session, pat, agt)", c.Kind))
	}

	if len(missing) > 0 {
		return fmt.Errorf("validation failed: invalid claims: %s", strings.Join(missing, ", "))
	}

	return nil
}
