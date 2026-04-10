package authn

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type testContextKey string

func TestClaimsRoundTrip(t *testing.T) {
	ctx := context.Background()
	claims := &Claims{Subject: "foo"}

	newCtx := ContextWithClaims(ctx, claims)

	cc, err := ClaimsFromContext(newCtx)
	assert.NoError(t, err)
	assert.Equal(t, "foo", cc.Subject)
}

func TestContextWithAnonymousClaims(t *testing.T) {
	t.Run("creates context with anonymous claims", func(t *testing.T) {
		ctx := ContextWithAnonymousClaims(context.Background())
		cc, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, cc.Subject)
	})

	t.Run("preserves parent context values", func(t *testing.T) {
		parentCtx := context.WithValue(context.Background(), testContextKey("key"), "value")
		ctx := ContextWithAnonymousClaims(parentCtx)

		assert.NotNil(t, ctx)
		assert.Equal(t, "value", ctx.Value(testContextKey("key")))
	})

	t.Run("creates independent instances", func(t *testing.T) {
		baseCtx := context.Background()
		ctx1 := ContextWithAnonymousClaims(baseCtx)
		ctx2 := ContextWithAnonymousClaims(baseCtx)

		claims1, err1 := ClaimsFromContext(ctx1)
		claims2, err2 := ClaimsFromContext(ctx2)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotSame(t, claims1, claims2)
		assert.Equal(t, claims1.Subject, claims2.Subject)
	})

	t.Run("anonymous claims have correct structure", func(t *testing.T) {
		ctx := ContextWithAnonymousClaims(context.Background())
		claims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, AnonymousSubject, claims.Subject)
		assert.Empty(t, claims.Kind)
		assert.Empty(t, claims.Scopes)
	})
}

func TestContextWithClaims(t *testing.T) {
	validUUID := uuid.New().String()

	testCases := []struct {
		name   string
		claims *Claims
	}{
		{
			name:   "valid user claims",
			claims: &Claims{Subject: validUUID, Kind: "pat"},
		},
		{
			name:   "claims with scopes",
			claims: &Claims{Subject: validUUID, Kind: "pat", Scopes: []string{"app:read", "env:write"}},
		},
		{
			name:   "minimal claims",
			claims: &Claims{Subject: validUUID},
		},
		{
			name:   "nil claims",
			claims: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			baseCtx := context.Background()
			ctx := ContextWithClaims(baseCtx, tc.claims)

			assert.NotNil(t, ctx)
			assert.NotEqual(t, baseCtx, ctx)

			retrievedClaims, err := ClaimsFromContext(ctx)
			if tc.claims == nil {
				assert.NoError(t, err)
				assert.Nil(t, retrievedClaims)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.claims, retrievedClaims)
				assert.Same(t, tc.claims, retrievedClaims)
			}
		})
	}

	t.Run("preserves parent context values", func(t *testing.T) {
		parentCtx := context.WithValue(context.Background(), testContextKey("parent-key"), "parent-value")
		claims := &Claims{Subject: validUUID, Kind: "pat"}

		ctx := ContextWithClaims(parentCtx, claims)

		assert.Equal(t, "parent-value", ctx.Value(testContextKey("parent-key")))

		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, claims, retrievedClaims)
	})

	t.Run("overwrites existing claims", func(t *testing.T) {
		ctx := ContextWithAnonymousClaims(context.Background())

		anonymousClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, AnonymousSubject, anonymousClaims.Subject)

		newClaims := &Claims{Subject: validUUID, Kind: "pat"}
		ctx = ContextWithClaims(ctx, newClaims)

		retrievedClaims, err := ClaimsFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, newClaims, retrievedClaims)
		assert.NotEqual(t, AnonymousSubject, retrievedClaims.Subject)
	})
}

func TestClaimsFromContext(t *testing.T) {
	validUUID := uuid.New().String()

	t.Run("retrieves valid claims", func(t *testing.T) {
		originalClaims := &Claims{Subject: validUUID, Kind: "pat", Scopes: []string{"app:read"}}

		ctx := ContextWithClaims(context.Background(), originalClaims)
		retrievedClaims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, retrievedClaims)
		assert.Same(t, originalClaims, retrievedClaims)
	})

	t.Run("retrieves anonymous claims", func(t *testing.T) {
		ctx := ContextWithAnonymousClaims(context.Background())
		claims, err := ClaimsFromContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, AnonymousSubject, claims.Subject)
	})

	t.Run("returns error when claims are wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, "not-claims")
		claims, err := ClaimsFromContext(ctx)

		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("returns error when no claims in context", func(t *testing.T) {
		cc, err := ClaimsFromContext(context.Background())
		assert.Nil(t, cc)
		assert.Error(t, err)
	})

	t.Run("returns error for nil context", func(t *testing.T) {
		cc, err := ClaimsFromContext(nil) //nolint:staticcheck
		assert.Nil(t, cc)
		assert.Error(t, err)
	})
}

func TestAnonymousSubjectConstant(t *testing.T) {
	assert.Equal(t, "system:anonymous", AnonymousSubject)
}
