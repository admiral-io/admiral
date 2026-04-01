package authn

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ParseTokenClaims parses a JWT token and extracts the claims.
func ParseTokenClaims(rawToken string, signingKey string) (*Claims, error) {
	if rawToken == "" {
		return nil, errors.New("raw token is empty")
	}

	if signingKey == "" {
		return nil, errors.New("signing key is not configured")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("invalid signing method: expected HS256")
		}
		return []byte(signingKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	if err := claims.Validate(); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	return claims, nil
}

// ParseTokenWithoutValidation parses a JWT token without validating the signature.
func ParseTokenWithoutValidation(rawToken string) (*Claims, error) {
	if rawToken == "" {
		return nil, errors.New("raw token is empty")
	}

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := &Claims{}

	_, _, err := parser.ParseUnverified(rawToken, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if err := claims.Validate(); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	return claims, nil
}
