package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseSetDefaults(t *testing.T) {
	d := &Database{}
	d.SetDefaults()

	assert.Equal(t, 5432, d.Port)
	assert.Equal(t, "admiral", d.DatabaseName)
	assert.Equal(t, SSLModeRequire, d.SSLMode)
	assert.Equal(t, 100, d.MaxOpenConns)
	assert.Equal(t, 10, d.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, d.ConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, d.ConnMaxIdleTime)
	assert.Equal(t, 5*time.Second, d.ConnectionTimeout)
}

func TestDatabaseSetDefaults_PreservesExisting(t *testing.T) {
	d := &Database{
		Port:         3306,
		DatabaseName: "custom",
		SSLMode:      SSLModeDisable,
		MaxOpenConns: 50,
	}
	d.SetDefaults()

	assert.Equal(t, 3306, d.Port)
	assert.Equal(t, "custom", d.DatabaseName)
	assert.Equal(t, SSLModeDisable, d.SSLMode)
	assert.Equal(t, 50, d.MaxOpenConns)
}

func TestDatabaseSetDefaults_NilReceiver(t *testing.T) {
	var d *Database
	d.SetDefaults() // should not panic
}

func TestDatabaseValidate(t *testing.T) {
	valid := &Database{Host: "localhost", User: "admin", Password: "secret", SSLMode: SSLModeDisable}

	tests := []struct {
		name    string
		db      *Database
		wantErr string
	}{
		{name: "nil", db: nil},
		{name: "valid", db: valid},
		{name: "missing host", db: &Database{User: "u", Password: "p", SSLMode: SSLModeDisable}, wantErr: "host is required"},
		{name: "missing user", db: &Database{Host: "h", Password: "p", SSLMode: SSLModeDisable}, wantErr: "user is required"},
		{name: "missing password", db: &Database{Host: "h", User: "u", SSLMode: SSLModeDisable}, wantErr: "password is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.db.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSLMode_String(t *testing.T) {
	tests := []struct {
		mode SSLMode
		want string
	}{
		{SSLModeUnspecified, "unspecified"},
		{SSLModeDisable, "disable"},
		{SSLModeAllow, "allow"},
		{SSLModePrefer, "prefer"},
		{SSLModeRequire, "require"},
		{SSLModeVerifyCA, "verify_ca"},
		{SSLModeVerifyFull, "verify_full"},
		{SSLMode(99), "unspecified"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mode.String())
		})
	}
}

func TestSSLMode_String_Nil(t *testing.T) {
	var s *SSLMode
	assert.Equal(t, "unspecified", s.String())
}

func TestSSLMode_Validate(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var s *SSLMode
		assert.Error(t, s.Validate())
	})

	t.Run("valid modes", func(t *testing.T) {
		for mode := SSLModeUnspecified; mode <= SSLModeVerifyFull; mode++ {
			assert.NoError(t, mode.Validate())
		}
	})

	t.Run("invalid mode", func(t *testing.T) {
		invalid := SSLMode(99)
		assert.Error(t, invalid.Validate())
	})
}
