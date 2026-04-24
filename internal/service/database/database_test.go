package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.admiral.io/admiral/internal/config"
)

func TestConnString(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Database
		want    string
		wantErr string
	}{
		{
			name: "valid config",
			cfg: &config.Database{
				Host:         "localhost",
				Port:         5432,
				DatabaseName: "admiral",
				User:         "admin",
				Password:     "secret",
				SSLMode:      config.SSLModeDisable,
			},
			want: "host=localhost port=5432 dbname=admiral user=admin password=secret connect_timeout=10 application_name=admiral sslmode=disable",
		},
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "no connection information",
		},
		{
			name: "empty host",
			cfg: &config.Database{
				Host:         "",
				DatabaseName: "admiral",
			},
			wantErr: "invalid host",
		},
		{
			name: "host with spaces",
			cfg: &config.Database{
				Host:         "bad host",
				DatabaseName: "admiral",
			},
			wantErr: "invalid host",
		},
		{
			name: "host with quotes",
			cfg: &config.Database{
				Host:         `host"injection`,
				DatabaseName: "admiral",
			},
			wantErr: "invalid host",
		},
		{
			name: "host with backslash",
			cfg: &config.Database{
				Host:         `host\injection`,
				DatabaseName: "admiral",
			},
			wantErr: "invalid host",
		},
		{
			name: "host with equals",
			cfg: &config.Database{
				Host:         "host=injection",
				DatabaseName: "admiral",
			},
			wantErr: "invalid host",
		},
		{
			name: "empty database name",
			cfg: &config.Database{
				Host:         "localhost",
				DatabaseName: "",
			},
			wantErr: "database name is required",
		},
		{
			name: "invalid ssl mode",
			cfg: &config.Database{
				Host:         "localhost",
				DatabaseName: "admiral",
				SSLMode:      config.SSLMode(99),
			},
			wantErr: "invalid SSLMode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConnString(tt.cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestConnString_SSLModes(t *testing.T) {
	base := &config.Database{
		Host:         "localhost",
		Port:         5432,
		DatabaseName: "admiral",
		User:         "admin",
		Password:     "secret",
	}

	tests := []struct {
		mode    config.SSLMode
		wantSSL string
	}{
		{config.SSLModeUnspecified, "sslmode=disable"},
		{config.SSLModeDisable, "sslmode=disable"},
		{config.SSLModeAllow, "sslmode=allow"},
		{config.SSLModePrefer, "sslmode=prefer"},
		{config.SSLModeRequire, "sslmode=require"},
		{config.SSLModeVerifyCA, "sslmode=verify-ca"},
		{config.SSLModeVerifyFull, "sslmode=verify-full"},
	}

	for _, tt := range tests {
		t.Run(tt.wantSSL, func(t *testing.T) {
			cfg := *base
			cfg.SSLMode = tt.mode
			got, err := ConnString(&cfg)
			require.NoError(t, err)
			assert.Contains(t, got, tt.wantSSL)
		})
	}
}

func TestName(t *testing.T) {
	assert.Equal(t, "service.database", Name)
}
