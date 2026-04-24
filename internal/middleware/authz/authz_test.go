package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"

	"go.admiral.io/admiral/internal/service/authn"
)

func TestCheckTokenType(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		kind    string
		wantErr bool
	}{
		{name: "no restriction", allowed: nil, kind: "pat"},
		{name: "empty allowed list", allowed: []string{}, kind: "pat"},
		{name: "allowed kind", allowed: []string{"pat", "session"}, kind: "pat"},
		{name: "session allowed", allowed: []string{"session"}, kind: "session"},
		{name: "denied kind", allowed: []string{"session"}, kind: "pat", wantErr: true},
		{name: "sat denied", allowed: []string{"pat"}, kind: "sat", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &commonv1.AuthRule{AllowedTokenTypes: tt.allowed}
			claims := &authn.Claims{Kind: tt.kind}

			err := checkTokenType(rule, claims)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, codes.PermissionDenied, status.Code(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckScope(t *testing.T) {
	tests := []struct {
		name     string
		required string
		scopes   []string
		wantErr  bool
	}{
		{name: "no scope required", required: "", scopes: nil},
		{name: "exact match", required: "app:read", scopes: []string{"app:read"}},
		{name: "among multiple", required: "env:write", scopes: []string{"app:read", "env:write"}},
		{name: "missing scope", required: "app:write", scopes: []string{"app:read"}, wantErr: true},
		{name: "empty scopes", required: "app:read", scopes: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &commonv1.AuthRule{Scope: tt.required}
			claims := &authn.Claims{Scopes: tt.scopes}

			err := checkScope(rule, claims)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, codes.PermissionDenied, status.Code(err))
				assert.Contains(t, err.Error(), tt.required)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHasScope(t *testing.T) {
	tests := []struct {
		name     string
		granted  []string
		required string
		want     bool
	}{
		{name: "exact match", granted: []string{"app:read"}, required: "app:read", want: true},
		{name: "wildcard match", granted: []string{"app:*"}, required: "app:read", want: true},
		{name: "wildcard write", granted: []string{"app:*"}, required: "app:write", want: true},
		{name: "wildcard wrong resource", granted: []string{"app:*"}, required: "env:read", want: false},
		{name: "no match", granted: []string{"env:read"}, required: "app:read", want: false},
		{name: "empty granted", granted: nil, required: "app:read", want: false},
		{name: "multiple with match", granted: []string{"env:read", "app:*"}, required: "app:write", want: true},
		{name: "multiple wildcards", granted: []string{"app:*", "env:*"}, required: "env:write", want: true},
		{name: "partial prefix no match", granted: []string{"app:re"}, required: "app:read", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasScope(tt.granted, tt.required))
		})
	}
}
