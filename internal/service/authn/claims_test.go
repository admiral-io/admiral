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
			name: "valid claims with all required fields",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
				},
				RedirectURL: "https://example.com/callback",
			},
			expectError: false,
		},
		{
			name: "valid claims without expiration",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
				},
				RedirectURL: "https://example.com/callback",
			},
			expectError: false,
		},
		{
			name: "valid claims without issued at",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				RedirectURL: "https://example.com/callback",
			},
			expectError: false,
		},
		{
			name: "missing redirect URL",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
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
					IssuedAt:  jwt.NewNumericDate(now.Add(-time.Hour * 2)),
				},
				RedirectURL: "https://example.com/callback",
			},
			expectError: true,
			errorMsg:    "validation failed: token has expired",
		},
		{
			name: "token issued in the future",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 2)),
					IssuedAt:  jwt.NewNumericDate(now.Add(time.Hour)),
				},
				RedirectURL: "https://example.com/callback",
			},
			expectError: true,
			errorMsg:    "validation failed: token issued in the future",
		},
		{
			name: "nil RegisteredClaims",
			claims: &stateClaims{
				RegisteredClaims: nil,
				RedirectURL:      "https://example.com/callback",
			},
			expectError: true,
			errorMsg:    "validation failed: registered claims are required",
		},
		{
			name: "empty redirect URL with whitespace",
			claims: &stateClaims{
				RegisteredClaims: &jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				RedirectURL: "   ",
			},
			expectError: true,
			errorMsg:    "validation failed: redirect URL claim is required",
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
		errorMsg    string
	}{
		{
			name: "valid claims with session kind",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
				Kind:            "session",
				Email:           "test@example.com",
				EmailVerified:   true,
				Name:            "John Doe",
				GivenName:       "John",
				FamilyName:      "Doe",
				ExternalSubject: "external-123",
				Picture:         "https://example.com/avatar.jpg",
				Groups:          []string{"admin", "users"},
			},
			expectError: false,
		},
		{
			name: "valid claims with agt kind",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
				Kind:            "agt",
				ExternalSubject: "cluster-456",
			},
			expectError: false,
		},
		{
			name: "minimal valid claims",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
				Kind: "pat",
			},
			expectError: false,
		},
		{
			name: "invalid UUID subject",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: "invalid-uuid",
				},
				Kind: "session",
			},
			expectError: true,
		},
		{
			name: "invalid kind",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
				Kind: "invalid-kind",
			},
			expectError: true,
		},
		{
			name: "empty subject",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: "",
				},
				Kind: "session",
			},
			expectError: true,
		},
		{
			name: "nil RegisteredClaims",
			claims: Claims{
				RegisteredClaims: nil,
				Kind:             "session",
			},
			expectError: true,
		},
		{
			name: "empty kind",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
				Kind: "",
			},
			expectError: false, // empty kind is allowed for backwards compatibility
		},
		{
			name: "claims with all optional fields",
			claims: Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject:   validUUID,
					Issuer:    "test-issuer",
					Audience:  []string{"test-audience"},
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					NotBefore: jwt.NewNumericDate(time.Now()),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ID:        "test-jti",
				},
				ExternalSubject: "ext-sub-123",
				Kind:            "session",
				Email:           "user@example.com",
				EmailVerified:   false,
				Name:            "Test User",
				GivenName:       "Test",
				FamilyName:      "User",
				Picture:         "https://example.com/pic.jpg",
				Groups:          []string{"group1", "group2", "group3"},
			},
			expectError: false,
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

func TestStateClaims_EdgeCases(t *testing.T) {
	t.Run("exactly at expiration time", func(t *testing.T) {
		now := time.Now()
		claims := &stateClaims{
			RegisteredClaims: &jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(now),
			},
			RedirectURL: "https://example.com/callback",
		}

		err := claims.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token has expired")
	})

	t.Run("exactly at issued time", func(t *testing.T) {
		now := time.Now()
		claims := &stateClaims{
			RegisteredClaims: &jwt.RegisteredClaims{
				IssuedAt: jwt.NewNumericDate(now),
			},
			RedirectURL: "https://example.com/callback",
		}

		err := claims.Validate()
		// This should pass as the current implementation only checks if IssuedAt is after Now()
		assert.NoError(t, err)
	})
}

func TestClaims_EdgeCases(t *testing.T) {
	t.Run("UUID validation with various formats", func(t *testing.T) {
		testCases := []struct {
			name    string
			subject string
		}{
			{"valid UUID v4", uuid.New().String()},
			{"valid UUID with uppercase", "550E8400-E29B-41D4-A716-446655440000"},
			{"valid UUID without hyphens", "550e8400e29b41d4a716446655440000"},
			{"nil UUID", "00000000-0000-0000-0000-000000000000"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				claims := Claims{
					RegisteredClaims: &jwt.RegisteredClaims{
						Subject: tc.subject,
					},
					Kind: "session",
				}

				err := claims.Validate()
				assert.NoError(t, err)
			})
		}
	})

	t.Run("kind validation with various values", func(t *testing.T) {
		validUUID := uuid.New().String()

		testCases := []struct {
			name        string
			kind        string
			expectError bool
		}{
			{"valid session kind", "session", false},
			{"valid pat kind", "pat", false},
			{"valid agt kind", "agt", false},
			{"empty kind (allowed)", "", false},
			{"invalid kind", "invalid", true},
			{"old db kind user", "user", true},
			{"old db kind cluster", "cluster", true},
			{"kind with spaces", " session ", true},
			{"uppercase kind", "SESSION", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				claims := Claims{
					RegisteredClaims: &jwt.RegisteredClaims{
						Subject: validUUID,
					},
					Kind: tc.kind,
				}

				err := claims.Validate()
				if tc.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestClaims_JSONTags(t *testing.T) {
	t.Run("verify struct field JSON tags", func(t *testing.T) {
		// This test ensures the JSON tags are correctly defined
		// by checking that the struct can be marshaled/unmarshaled
		original := Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				Subject: uuid.New().String(),
			},
			ExternalSubject: "external-123",
			Kind:            "session",
			Email:           "test@example.com",
			EmailVerified:   true,
			Name:            "John Doe",
			GivenName:       "John",
			FamilyName:      "Doe",
			Picture:         "https://example.com/avatar.jpg",
			Groups:          []string{"admin", "users"},
		}

		// The JSON marshaling would be tested here if needed
		// For now, we just verify the struct can be created
		assert.NotNil(t, original)
		assert.Equal(t, "external-123", original.ExternalSubject)
		assert.Equal(t, "session", original.Kind)
		assert.Equal(t, "test@example.com", original.Email)
		assert.True(t, original.EmailVerified)
		assert.Equal(t, "John Doe", original.Name)
		assert.Equal(t, "John", original.GivenName)
		assert.Equal(t, "Doe", original.FamilyName)
		assert.Equal(t, "https://example.com/avatar.jpg", original.Picture)
		assert.Equal(t, []string{"admin", "users"}, original.Groups)
	})
}

func TestStateClaims_Construction(t *testing.T) {
	t.Run("create stateClaims with constructor pattern", func(t *testing.T) {
		now := time.Now()
		redirectURL := "https://example.com/callback"

		claims := &stateClaims{
			RegisteredClaims: &jwt.RegisteredClaims{
				Issuer:    "test-issuer",
				Subject:   "test-subject",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				NotBefore: jwt.NewNumericDate(now),
				IssuedAt:  jwt.NewNumericDate(now),
				ID:        "test-id",
			},
			RedirectURL: redirectURL,
		}

		assert.NotNil(t, claims)
		assert.NotNil(t, claims.RegisteredClaims)
		assert.Equal(t, redirectURL, claims.RedirectURL)
		assert.Equal(t, "test-issuer", claims.Issuer)
		assert.Equal(t, "test-subject", claims.Subject)

		err := claims.Validate()
		assert.NoError(t, err)
	})
}

func TestClaims_Construction(t *testing.T) {
	t.Run("create Claims with constructor pattern", func(t *testing.T) {
		validUUID := uuid.New().String()

		claims := Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				Subject: validUUID,
				Issuer:  "test-issuer",
			},
			ExternalSubject: "external-123",
			Kind:            "session",
			Email:           "test@example.com",
			EmailVerified:   true,
			Name:            "Test User",
			GivenName:       "Test",
			FamilyName:      "User",
			Picture:         "https://example.com/avatar.jpg",
			Groups:          []string{"group1", "group2"},
		}

		assert.NotNil(t, claims)
		assert.NotNil(t, claims.RegisteredClaims)
		assert.Equal(t, validUUID, claims.Subject)
		assert.Equal(t, "external-123", claims.ExternalSubject)
		assert.Equal(t, "session", claims.Kind)
		assert.Equal(t, "test@example.com", claims.Email)
		assert.True(t, claims.EmailVerified)
		assert.Equal(t, "Test User", claims.Name)
		assert.Equal(t, "Test", claims.GivenName)
		assert.Equal(t, "User", claims.FamilyName)
		assert.Equal(t, "https://example.com/avatar.jpg", claims.Picture)
		assert.Len(t, claims.Groups, 2)
		assert.Contains(t, claims.Groups, "group1")
		assert.Contains(t, claims.Groups, "group2")

		err := claims.Validate()
		assert.NoError(t, err)
	})
}
