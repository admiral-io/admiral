package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthnSetDefaults(t *testing.T) {
	a := &Authn{}
	a.SetDefaults()

	assert.Equal(t, []string{"openid", "email", "profile", "offline_access"}, a.Scopes)
	assert.Equal(t, "sub", a.SubjectClaim)
	assert.Equal(t, 12*time.Hour, a.SessionRefreshTTL)
}

func TestAuthnSetDefaults_PreservesExisting(t *testing.T) {
	a := &Authn{
		Scopes:            []string{"openid"},
		SubjectClaim:      "preferred_username",
		SessionRefreshTTL: time.Hour,
	}
	a.SetDefaults()

	assert.Equal(t, []string{"openid"}, a.Scopes)
	assert.Equal(t, "preferred_username", a.SubjectClaim)
	assert.Equal(t, time.Hour, a.SessionRefreshTTL)
}

func TestAuthnSetDefaults_NilReceiver(t *testing.T) {
	var a *Authn
	a.SetDefaults() // should not panic
}

func validAuthn() *Authn {
	return &Authn{
		Issuer:        "https://idp.example.com",
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		RedirectURL:   "http://localhost:8080/callback",
		SigningSecret:  "this-is-at-least-32-bytes-long!!",
	}
}

func TestAuthnValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Authn)
		wantErr string
	}{
		{name: "valid", modify: nil},
		{name: "missing issuer", modify: func(a *Authn) { a.Issuer = "" }, wantErr: "issuer is required"},
		{name: "missing client_id", modify: func(a *Authn) { a.ClientID = "" }, wantErr: "client_id is required"},
		{name: "missing client_secret", modify: func(a *Authn) { a.ClientSecret = "" }, wantErr: "client_secret is required"},
		{name: "missing redirect_url", modify: func(a *Authn) { a.RedirectURL = "" }, wantErr: "redirect_url is required"},
		{name: "short signing_secret", modify: func(a *Authn) { a.SigningSecret = "short" }, wantErr: "signing_secret must be at least 32 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := validAuthn()
			if tt.modify != nil {
				tt.modify(a)
			}
			err := a.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthnValidate_Nil(t *testing.T) {
	var a *Authn
	assert.NoError(t, a.Validate())
}
