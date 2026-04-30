package authn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidTokenKind(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"SESSION", true},
		{"PAT", true},
		{"SAT", true},
		{"", false},
		{"invalid", false},
		{"pat", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidTokenKind(tt.kind))
		})
	}
}

func TestValidateScopes(t *testing.T) {
	t.Run("valid scopes", func(t *testing.T) {
		assert.NoError(t, ValidateScopes([]string{"app:read", "env:write"}))
	})

	t.Run("empty scopes", func(t *testing.T) {
		assert.NoError(t, ValidateScopes(nil))
		assert.NoError(t, ValidateScopes([]string{}))
	})

	t.Run("invalid scope", func(t *testing.T) {
		err := ValidateScopes([]string{"app:read", "bogus:scope"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bogus:scope")
	})

	t.Run("wildcard scopes are not valid as token scopes", func(t *testing.T) {
		err := ValidateScopes([]string{"app:*"})
		assert.Error(t, err)
	})

	t.Run("all defined scopes are valid", func(t *testing.T) {
		assert.NoError(t, ValidateScopes(AllScopes))
	})
}

func TestTokenKindConstants(t *testing.T) {
	assert.Equal(t, TokenKind("SESSION"), TokenKindSession)
	assert.Equal(t, TokenKind("PAT"), TokenKindPAT)
	assert.Equal(t, TokenKind("SAT"), TokenKindSAT)
}
