package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const migrateOnlyYAML = `
services:
  database:
    host: localhost
    port: 5432
    user: admiral
    password: secret
    database_name: admiral
    ssl_mode: disable
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// Load + ValidateRequired(Database) succeed on a config that has only the
// database block -- the migration command's path must not require encryption
// or object storage.
func TestLoad_MigrationOnlyConfig(t *testing.T) {
	path := writeConfig(t, migrateOnlyYAML)

	cfg, err := Load(path, nil, false)
	require.NoError(t, err)
	require.NotNil(t, cfg.Services.Database)

	assert.NoError(t, ValidateRequired(cfg.Services.Database, "services.database"))
}

// Build rejects the same minimal config because it enforces the full server
// surface (encryption, object storage required).
func TestBuild_RejectsMigrationOnlyConfig(t *testing.T) {
	path := writeConfig(t, migrateOnlyYAML)

	_, err := Build(path, nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
}

func TestValidateRequired_NilConfigurable(t *testing.T) {
	var d *Database
	err := ValidateRequired(d, "services.database")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "services.database config is nil")
}
