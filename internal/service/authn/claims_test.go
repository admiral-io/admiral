package authn

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStateClaims_Validate(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name        string
		claims      *stateClaims
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid claims",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
				},
				RedirectURL: "/callback",
			},
		},
		{
			name: "missing redirect URL",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				RedirectURL: "",
			},
			expectError: true,
			errorMsg:    "validation failed: redirect URL claim is required",
		},
		{
			name: "expired token",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
				},
				RedirectURL: "/callback",
			},
			expectError: true,
			errorMsg:    "validation failed: token has expired",
		},
		{
			name: "token issued in the future",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					IssuedAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				RedirectURL: "/callback",
			},
			expectError: true,
			errorMsg:    "validation failed: token issued in the future",
		},
		{
			name: "nil RegisteredClaims",
			claims: &stateClaims{
				RegisteredClaims: nil,
				RedirectURL:      "/callback",
			},
			expectError: true,
			errorMsg:    "validation failed: registered claims are required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.claims.Validate()
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Equal(t, tc.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClaims_Validate(t *testing.T) {
	validUUID := uuid.New().String()

	testCases := []struct {
		name        string
		claims      Claims
		expectError bool
	}{
		{
			name:   "valid session claims",
			claims: Claims{Subject: validUUID, Kind: "session"},
		},
		{
			name:   "valid pat claims",
			claims: Claims{Subject: validUUID, Kind: "pat"},
		},
		{
			name:   "valid sat claims",
			claims: Claims{Subject: validUUID, Kind: "sat"},
		},
		{
			name:   "empty kind allowed",
			claims: Claims{Subject: validUUID, Kind: ""},
		},
		{
			name:   "claims with scopes",
			claims: Claims{Subject: validUUID, Kind: "pat", Scopes: []string{"app:read"}},
		},
		{
			name:        "invalid UUID subject",
			claims:      Claims{Subject: "invalid-uuid", Kind: "session"},
			expectError: true,
		},
		{
			name:        "empty subject",
			claims:      Claims{Subject: "", Kind: "session"},
			expectError: true,
		},
		{
			name:        "invalid kind",
			claims:      Claims{Subject: validUUID, Kind: "invalid"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.claims.Validate()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
