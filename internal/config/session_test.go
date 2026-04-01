package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionSetDefaults(t *testing.T) {
	s := &Session{}
	s.SetDefaults()

	assert.Equal(t, 24*time.Hour, s.Lifetime)
	assert.Equal(t, 20*time.Minute, s.IdleTimeout)
	assert.Equal(t, "session", s.Cookie.Name)
	assert.Equal(t, SessionSameSiteLax, s.Cookie.SameSite)
	require.NotNil(t, s.Cookie.HttpOnly)
	assert.True(t, *s.Cookie.HttpOnly)
	require.NotNil(t, s.Cookie.Secure)
	assert.False(t, *s.Cookie.Secure)
	require.NotNil(t, s.Cookie.Persist)
	assert.True(t, *s.Cookie.Persist)
}

func TestSessionSetDefaults_PreservesExisting(t *testing.T) {
	f := false
	s := &Session{
		Lifetime:    2 * time.Hour,
		IdleTimeout: 10 * time.Minute,
		Cookie: Cookie{
			Name:     "custom",
			SameSite: SessionSameSiteStrict,
			HttpOnly: &f,
			Secure:   &f,
		},
	}
	s.SetDefaults()

	assert.Equal(t, 2*time.Hour, s.Lifetime)
	assert.Equal(t, 10*time.Minute, s.IdleTimeout)
	assert.Equal(t, "custom", s.Cookie.Name)
	assert.Equal(t, SessionSameSiteStrict, s.Cookie.SameSite)
	assert.False(t, *s.Cookie.HttpOnly)
	assert.False(t, *s.Cookie.Secure)
}

func TestSessionSetDefaults_NilReceiver(t *testing.T) {
	var s *Session
	// Should not panic.
	s.SetDefaults()
}

func TestSessionValidate(t *testing.T) {
	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{name: "nil session", session: nil, wantErr: false},
		{name: "empty SameSite", session: &Session{}, wantErr: false},
		{name: "lax", session: &Session{Cookie: Cookie{SameSite: SessionSameSiteLax}}, wantErr: false},
		{name: "strict", session: &Session{Cookie: Cookie{SameSite: SessionSameSiteStrict}}, wantErr: false},
		{name: "none", session: &Session{Cookie: Cookie{SameSite: SessionSameSiteNone}}, wantErr: false},
		{name: "invalid", session: &Session{Cookie: Cookie{SameSite: "bogus"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSameSiteConstants(t *testing.T) {
	assert.Equal(t, SameSiteMode("lax"), SessionSameSiteLax)
	assert.Equal(t, SameSiteMode("strict"), SessionSameSiteStrict)
	assert.Equal(t, SameSiteMode("none"), SessionSameSiteNone)
}
