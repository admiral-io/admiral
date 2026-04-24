package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApplicationStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewApplicationStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewComponentStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewComponentStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewCredentialStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewCredentialStore(nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewDeploymentStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewDeploymentStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewEnvironmentStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewEnvironmentStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewModuleStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewModuleStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewRunnerStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewRunnerStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewSourceStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewSourceStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewVariableStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewVariableStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewTokenStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewTokenStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewJobStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewJobStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewRevisionStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewRevisionStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}

func TestNewUserStore(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		_, err := NewUserStore(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database is required")
	})
}
