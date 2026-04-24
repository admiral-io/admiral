package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gopkg.in/yaml.v3"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	assert.Equal(t, zapcore.InfoLevel, cfg.Level)
	assert.False(t, cfg.Pretty)
	assert.Empty(t, cfg.Namespace)
}

func TestConfigBuilders(t *testing.T) {
	cfg := NewConfig().
		WithLevel(zapcore.DebugLevel).
		WithPretty(true).
		WithNamespace("test")

	assert.Equal(t, zapcore.DebugLevel, cfg.Level)
	assert.True(t, cfg.Pretty)
	assert.Equal(t, "test", cfg.Namespace)
}

func TestDevelopmentConfig(t *testing.T) {
	cfg := Development()
	assert.Equal(t, zapcore.DebugLevel, cfg.Level)
	assert.True(t, cfg.Pretty)
}

func TestProductionConfig(t *testing.T) {
	cfg := Production()
	assert.Equal(t, zapcore.InfoLevel, cfg.Level)
	assert.False(t, cfg.Pretty)
}

func TestConfigUnmarshalYAML(t *testing.T) {
	t.Run("defaults to error level", func(t *testing.T) {
		var cfg Config
		err := yaml.Unmarshal([]byte(`{}`), &cfg)
		require.NoError(t, err)
		assert.Equal(t, zap.ErrorLevel, cfg.Level)
	})

	t.Run("parses debug level", func(t *testing.T) {
		var cfg Config
		err := yaml.Unmarshal([]byte(`level: debug`), &cfg)
		require.NoError(t, err)
		assert.Equal(t, zapcore.DebugLevel, cfg.Level)
	})

	t.Run("parses pretty flag", func(t *testing.T) {
		var cfg Config
		err := yaml.Unmarshal([]byte("pretty: true"), &cfg)
		require.NoError(t, err)
		assert.True(t, cfg.Pretty)
	})
}

func TestNew(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		l, err := New(nil, "test")
		require.NoError(t, err)
		assert.NotNil(t, l)
	})

	t.Run("production config", func(t *testing.T) {
		l, err := New(Production(), "server")
		require.NoError(t, err)
		assert.NotNil(t, l)
	})

	t.Run("development config", func(t *testing.T) {
		l, err := New(Development(), "server")
		require.NoError(t, err)
		assert.NotNil(t, l)
	})

	t.Run("empty component", func(t *testing.T) {
		l, err := New(NewConfig(), "")
		require.NoError(t, err)
		assert.NotNil(t, l)
	})
}

func TestNewWithCore(t *testing.T) {
	t.Run("custom core captures output", func(t *testing.T) {
		core, logs := observer.New(zapcore.InfoLevel)
		l, err := NewWithCore(NewConfig(), "test", core)
		require.NoError(t, err)

		l.Info("hello")
		assert.Equal(t, 1, logs.Len())
		assert.Equal(t, "hello", logs.All()[0].Message)
	})

	t.Run("namespace groups fields", func(t *testing.T) {
		core, logs := observer.New(zapcore.InfoLevel)
		cfg := NewConfig().WithNamespace("ctx")
		l, err := NewWithCore(cfg, "", core)
		require.NoError(t, err)

		l.Info("msg", zap.String("key", "val"))
		assert.Equal(t, 1, logs.Len())
	})
}

func TestNewFromLevel(t *testing.T) {
	l, err := NewFromLevel(zapcore.WarnLevel, "test")
	require.NoError(t, err)
	assert.NotNil(t, l)
}

func TestNewSimple(t *testing.T) {
	l, err := NewSimple("test")
	require.NoError(t, err)
	assert.NotNil(t, l)
}

func TestNewBootstrap(t *testing.T) {
	l := NewBootstrap()
	assert.NotNil(t, l)
}

func TestNewNop(t *testing.T) {
	l := NewNop()
	assert.NotNil(t, l)
	// Should not panic on use.
	l.Info("discarded")
}

func TestMust(t *testing.T) {
	t.Run("returns logger on success", func(t *testing.T) {
		l := Must(NewSimple("test"))
		assert.NotNil(t, l)
	})

	t.Run("panics on error", func(t *testing.T) {
		assert.Panics(t, func() {
			Must(nil, assert.AnError)
		})
	})
}
