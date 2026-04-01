package authn

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTokenClaims(t *testing.T) {
	signingKey := "test-signing-key"

	t.Run("valid token", func(t *testing.T) {
		// Create a valid token
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				ID:        uuid.New().String(),
				Subject:   uuid.New().String(),
				Issuer:    "admiral",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			Kind:  "session",
			Email: "test@example.com",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(signingKey))
		require.NoError(t, err)

		// Parse the token
		parsedClaims, err := ParseTokenClaims(tokenString, signingKey)
		require.NoError(t, err)
		assert.Equal(t, claims.ID, parsedClaims.ID)
		assert.Equal(t, claims.Subject, parsedClaims.Subject)
		assert.Equal(t, claims.Kind, parsedClaims.Kind)
		assert.Equal(t, claims.Email, parsedClaims.Email)
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := ParseTokenClaims("", signingKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raw token is empty")
	})

	t.Run("empty signing key", func(t *testing.T) {
		_, err := ParseTokenClaims("some-token", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signing key is not configured")
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := ParseTokenClaims("invalid-token", signingKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse token")
	})

	t.Run("wrong signing method", func(t *testing.T) {
		// Create token with different signing method
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				ID:      uuid.New().String(),
				Subject: uuid.New().String(),
			},
			Kind: "session",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
		tokenString, err := token.SignedString([]byte(signingKey))
		require.NoError(t, err)

		_, err = ParseTokenClaims(tokenString, signingKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signing method")
	})
}

func TestParseTokenWithoutValidation(t *testing.T) {
	signingKey := "test-signing-key"

	t.Run("valid token", func(t *testing.T) {
		// Create a valid token
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				ID:        uuid.New().String(),
				Subject:   uuid.New().String(),
				Issuer:    "admiral",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			Kind:  "agt",
			Email: "test@example.com",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(signingKey))
		require.NoError(t, err)

		// Parse without validation
		parsedClaims, err := ParseTokenWithoutValidation(tokenString)
		require.NoError(t, err)
		assert.Equal(t, claims.ID, parsedClaims.ID)
		assert.Equal(t, claims.Subject, parsedClaims.Subject)
		assert.Equal(t, claims.Kind, parsedClaims.Kind)
		assert.Equal(t, claims.Email, parsedClaims.Email)
	})

	t.Run("token with different signing key", func(t *testing.T) {
		// Create a token with one key
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				ID:      uuid.New().String(),
				Subject: uuid.New().String(),
			},
			Kind: "agt",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("different-key"))
		require.NoError(t, err)

		// Should still parse without validation
		parsedClaims, err := ParseTokenWithoutValidation(tokenString)
		require.NoError(t, err)
		assert.Equal(t, claims.ID, parsedClaims.ID)
		assert.Equal(t, claims.Subject, parsedClaims.Subject)
		assert.Equal(t, claims.Kind, parsedClaims.Kind)
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := ParseTokenWithoutValidation("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raw token is empty")
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := ParseTokenWithoutValidation("not-a-jwt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse token")
	})

	t.Run("invalid claims", func(t *testing.T) {
		// Create token with invalid subject (not UUID)
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				ID:      uuid.New().String(),
				Subject: "not-a-uuid",
			},
			Kind: "agt",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(signingKey))
		require.NoError(t, err)

		_, err = ParseTokenWithoutValidation(tokenString)
		assert.Error(t, err)
	})
}
