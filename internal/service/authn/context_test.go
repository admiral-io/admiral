package authn

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestClaimsRoundTrip(t *testing.T) {
	ctx := context.Background()

	claims := &Claims{
		RegisteredClaims: &jwt.RegisteredClaims{Subject: "foo"},
	}

	newCtx := ContextWithClaims(ctx, claims)

	cc, err := ClaimsFromContext(newCtx)
	assert.NoError(t, err)
	assert.Equal(t, "foo", cc.Subject)
}

type testContextKey string

func TestContextWithAnonymousClaims(t *testing.T) {
	t.Run("creates context with anonymous claims", func(t *testing.T) {
		ctx := context.Background()
		ctx = ContextWithAnonymousClaims(ctx)
		cc, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, cc.Subject)
	})

	t.Run("works with non-background context", func(t *testing.T) {
		parentCtx := context.WithValue(context.Background(), testContextKey("key"), "value")
		ctx := ContextWithAnonymousClaims(parentCtx)

		assert.NotNil(t, ctx)

		// Verify parent context values are preserved
		assert.Equal(t, "value", ctx.Value(testContextKey("key")))

		// Verify anonymous claims are set
		claims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, claims.Subject)
	})

	t.Run("creates new claims instance each time", func(t *testing.T) {
		baseCtx := context.Background()
		ctx1 := ContextWithAnonymousClaims(baseCtx)
		ctx2 := ContextWithAnonymousClaims(baseCtx)

		claims1, err1 := ClaimsFromContext(ctx1)
		claims2, err2 := ClaimsFromContext(ctx2)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotSame(t, claims1, claims2)               // Different instances
		assert.Equal(t, claims1.Subject, claims2.Subject) // Same values
	})

	t.Run("anonymous claims have correct structure", func(t *testing.T) {
		ctx := ContextWithAnonymousClaims(context.Background())
		claims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.NotNil(t, claims.RegisteredClaims)
		assert.Equal(t, AnonymousSubject, claims.Subject)
		assert.Equal(t, "system:anonymous", claims.Subject)

		// Verify other fields are zero values
		assert.Empty(t, claims.Kind)
		assert.Empty(t, claims.Email)
		assert.False(t, claims.EmailVerified)
		assert.Empty(t, claims.Name)
		assert.Empty(t, claims.ExternalSubject)
		assert.Empty(t, claims.Groups)
	})
}

func TestContextWithClaims(t *testing.T) {
	validUUID := uuid.New().String()

	testCases := []struct {
		name   string
		claims *Claims
	}{
		{
			name: "valid user claims",
			claims: &Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
				Kind:            "user",
				Email:           "test@example.com",
				EmailVerified:   true,
				Name:            "Test User",
				ExternalSubject: "external-123",
			},
		},
		{
			name: "valid cluster claims",
			claims: &Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
				Kind:            "cluster",
				ExternalSubject: "cluster-456",
			},
		},
		{
			name: "minimal claims",
			claims: &Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject: validUUID,
				},
			},
		},
		{
			name:   "nil claims",
			claims: nil,
		},
		{
			name: "claims with all fields",
			claims: &Claims{
				RegisteredClaims: &jwt.RegisteredClaims{
					Subject:   validUUID,
					Issuer:    "test-issuer",
					Audience:  []string{"test-audience"},
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					NotBefore: jwt.NewNumericDate(time.Now()),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ID:        "test-jti",
				},
				ExternalSubject: "external-subject",
				Kind:            "user",
				Email:           "user@example.com",
				EmailVerified:   false,
				Name:            "Full Name",
				GivenName:       "Full",
				FamilyName:      "Name",
				Picture:         "https://example.com/picture.jpg",
				Groups:          []string{"group1", "group2", "group3"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			baseCtx := context.Background()
			ctx := ContextWithClaims(baseCtx, tc.claims)

			assert.NotNil(t, ctx)
			assert.NotEqual(t, baseCtx, ctx)

			// Retrieve and verify claims
			retrievedClaims, err := ClaimsFromContext(ctx)
			if tc.claims == nil {
				// When nil claims are stored, they are retrieved as nil but without error
				assert.NoError(t, err)
				assert.Nil(t, retrievedClaims)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.claims, retrievedClaims)
				assert.Same(t, tc.claims, retrievedClaims) // Same instance
			}
		})
	}

	t.Run("preserves parent context values", func(t *testing.T) {
		parentCtx := context.WithValue(context.Background(), testContextKey("parent-key"), "parent-value")
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID},
			Kind:             "user",
		}

		ctx := ContextWithClaims(parentCtx, claims)

		// Verify parent context value is preserved
		assert.Equal(t, "parent-value", ctx.Value(testContextKey("parent-key")))

		// Verify claims are set
		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, claims, retrievedClaims)
	})

	t.Run("overwrites existing claims", func(t *testing.T) {
		// Start with anonymous claims
		ctx := ContextWithAnonymousClaims(context.Background())

		// Verify anonymous claims are set
		anonymousClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, anonymousClaims.Subject)

		// Overwrite with new claims
		newClaims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID},
			Kind:             "user",
		}
		ctx = ContextWithClaims(ctx, newClaims)

		// Verify new claims have replaced anonymous claims
		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, newClaims, retrievedClaims)
		assert.NotEqual(t, AnonymousSubject, retrievedClaims.Subject)
	})
}

func TestClaimsFromContext(t *testing.T) {
	validUUID := uuid.New().String()

	t.Run("retrieves valid claims", func(t *testing.T) {
		originalClaims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID},
			Kind:             "user",
			Email:            "test@example.com",
		}

		ctx := ContextWithClaims(context.Background(), originalClaims)
		retrievedClaims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, retrievedClaims)
		assert.Equal(t, originalClaims, retrievedClaims)
		assert.Same(t, originalClaims, retrievedClaims) // Same instance
	})

	t.Run("retrieves anonymous claims", func(t *testing.T) {
		ctx := ContextWithAnonymousClaims(context.Background())
		claims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, AnonymousSubject, claims.Subject)
	})

	t.Run("returns error when claims are wrong type", func(t *testing.T) {
		// Set a string value instead of *Claims
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, "not-claims")
		claims, err := ClaimsFromContext(ctx)

		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Equal(t, "claims in context were not present or not the correct model", err.Error())
	})

	t.Run("returns error when claims are different struct type", func(t *testing.T) {
		// Set a different struct type
		type WrongClaims struct {
			Subject string
		}
		wrongClaims := &WrongClaims{Subject: "test"}
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, wrongClaims)

		claims, err := ClaimsFromContext(ctx)

		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Equal(t, "claims in context were not present or not the correct model", err.Error())
	})

	t.Run("handles nil claims stored in context", func(t *testing.T) {
		var nilClaims *Claims = nil
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, nilClaims)

		claims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.Nil(t, claims)
	})
}

func TestNilClaimsValueErrors(t *testing.T) {
	t.Run("returns error when no claims in context", func(t *testing.T) {
		cc, err := ClaimsFromContext(context.Background())
		assert.Nil(t, cc)
		assert.Error(t, err)
		assert.Equal(t, "claims in context were not present or not the correct model", err.Error())
	})

	t.Run("returns error for nil context", func(t *testing.T) {
		cc, err := ClaimsFromContext(nil) // nolint:staticcheck
		assert.Nil(t, cc)
		assert.Error(t, err)
		assert.Equal(t, "no context found when evaluating claims", err.Error())
	})
}

func TestClaimsContextKey(t *testing.T) {
	t.Run("ClaimsContextKey is unique", func(t *testing.T) {
		key1 := ClaimsContextKey{}
		key2 := ClaimsContextKey{}

		// Even though they're the same type, they should be considered equal
		// since it's an empty struct
		assert.Equal(t, key1, key2)

		// But they should work as context keys
		ctx1 := context.WithValue(context.Background(), key1, "value1")
		ctx2 := context.WithValue(ctx1, key2, "value2")

		// Since keys are equal, value2 should overwrite value1
		assert.Equal(t, "value2", ctx2.Value(key1))
		assert.Equal(t, "value2", ctx2.Value(key2))
	})

	t.Run("ClaimsContextKey works with different value types", func(t *testing.T) {
		key := ClaimsContextKey{}

		// Test that only *Claims type is properly handled by ClaimsFromContext
		testCases := []struct {
			name        string
			value       interface{}
			expectError bool
		}{
			{"valid claims", &Claims{}, false},
			{"nil claims", (*Claims)(nil), false},
			{"string value", "test", true},
			{"int value", 42, true},
			{"map value", map[string]string{"test": "value"}, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.WithValue(context.Background(), key, tc.value)
				claims, err := ClaimsFromContext(ctx)

				if tc.expectError {
					assert.Error(t, err)
					assert.Nil(t, claims)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.value, claims)
				}
			})
		}
	})
}

func TestAnonymousSubjectConstant(t *testing.T) {
	t.Run("AnonymousSubject has correct value", func(t *testing.T) {
		assert.Equal(t, "system:anonymous", AnonymousSubject)
	})

	t.Run("AnonymousSubject is used in ContextWithAnonymousClaims", func(t *testing.T) {
		ctx := ContextWithAnonymousClaims(context.Background())
		claims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, claims.Subject)
	})
}

func TestContextIntegration(t *testing.T) {
	t.Run("complete workflow with claims", func(t *testing.T) {
		validUUID := uuid.New().String()

		// Start with background context
		ctx := context.Background()

		// Add some parent values
		ctx = context.WithValue(ctx, testContextKey("request-id"), "req-123")
		ctx = context.WithValue(ctx, testContextKey("trace-id"), "trace-456")

		// Set anonymous claims first
		ctx = ContextWithAnonymousClaims(ctx)

		// Verify anonymous claims
		claims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, claims.Subject)

		// Authenticate with real user claims
		userClaims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{
				Subject: validUUID,
				Issuer:  "test-issuer",
			},
			Kind:  "user",
			Email: "user@example.com",
			Name:  "Test User",
		}
		ctx = ContextWithClaims(ctx, userClaims)

		// Verify authenticated claims
		claims, err = ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, userClaims, claims)
		assert.NotEqual(t, AnonymousSubject, claims.Subject)

		// Verify parent context values are preserved
		assert.Equal(t, "req-123", ctx.Value(testContextKey("request-id")))
		assert.Equal(t, "trace-456", ctx.Value(testContextKey("trace-id")))
	})

	t.Run("context chain with multiple claims updates", func(t *testing.T) {
		validUUID1 := uuid.New().String()
		validUUID2 := uuid.New().String()

		ctx := context.Background()

		// First user
		claims1 := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID1},
			Kind:             "user",
			Email:            "user1@example.com",
		}
		ctx = ContextWithClaims(ctx, claims1)

		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "user1@example.com", retrievedClaims.Email)

		// Switch to second user
		claims2 := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID2},
			Kind:             "user",
			Email:            "user2@example.com",
		}
		ctx = ContextWithClaims(ctx, claims2)

		retrievedClaims, err = ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "user2@example.com", retrievedClaims.Email)
		assert.NotEqual(t, validUUID1, retrievedClaims.Subject)
		assert.Equal(t, validUUID2, retrievedClaims.Subject)

		// Switch back to anonymous
		ctx = ContextWithAnonymousClaims(ctx)

		retrievedClaims, err = ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, retrievedClaims.Subject)
	})

	t.Run("context cancellation behavior", func(t *testing.T) {
		validUUID := uuid.New().String()

		// Create a cancellable context with claims
		ctx, cancel := context.WithCancel(context.Background())
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID},
			Kind:             "user",
		}
		ctx = ContextWithClaims(ctx, claims)

		// Verify claims work before cancellation
		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, validUUID, retrievedClaims.Subject)

		// Cancel the context
		cancel()

		// Claims should still be accessible even after cancellation
		retrievedClaims, err = ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, validUUID, retrievedClaims.Subject)

		// But context should be cancelled
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	})
}

func TestContextEdgeCases(t *testing.T) {
	t.Run("deeply nested context values", func(t *testing.T) {
		validUUID := uuid.New().String()

		// Create deeply nested context
		ctx := context.Background()
		type testIntKey int
		for i := 0; i < 10; i++ {
			ctx = context.WithValue(ctx, testIntKey(i), i*2)
		}

		// Add claims
		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID},
			Kind:             "user",
		}
		ctx = ContextWithClaims(ctx, claims)

		// Verify claims are accessible
		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, validUUID, retrievedClaims.Subject)

		// Verify all parent values are still accessible
		for i := 0; i < 10; i++ {
			assert.Equal(t, i*2, ctx.Value(testIntKey(i)))
		}
	})

	t.Run("context with timeout", func(t *testing.T) {
		validUUID := uuid.New().String()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		claims := &Claims{
			RegisteredClaims: &jwt.RegisteredClaims{Subject: validUUID},
			Kind:             "user",
		}
		ctx = ContextWithClaims(ctx, claims)

		// Verify claims work with timeout context
		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, validUUID, retrievedClaims.Subject)

		// Verify deadline is preserved
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.True(t, deadline.After(time.Now()))
	})
}
