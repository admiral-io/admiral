package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a logger instance using the simple logger configuration
func New(cfg *Config, component string) (*zap.Logger, error) {
	return NewWithCore(cfg, component, nil)
}

// NewFromLevel creates a logger with just a log level and component
func NewFromLevel(level zapcore.Level, component string) (*zap.Logger, error) {
	cfg := &Config{
		Level:     level,
		Pretty:    false,
		Namespace: "",
	}
	return New(cfg, component)
}

// NewSimple creates a logger with sensible defaults (Info level, production mode)
func NewSimple(component string) (*zap.Logger, error) {
	return NewFromLevel(zapcore.InfoLevel, component)
}

// NewWithCore creates a logger with optional custom core (primarily for testing)
// This is the main logger creation function that all other functions delegate to
func NewWithCore(cfg *Config, component string, customCore zapcore.Core) (*zap.Logger, error) {
	var zapConfig zap.Config
	var opts []zap.Option

	if cfg != nil && cfg.Pretty {
		zapConfig = zap.NewDevelopmentConfig()
		opts = append(opts, zap.AddStacktrace(zap.ErrorLevel))
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	level := zap.NewAtomicLevel()
	if cfg != nil {
		level.SetLevel(cfg.Level)
	} else {
		level.SetLevel(zapcore.InfoLevel)
	}
	zapConfig.Level = level

	logger, err := zapConfig.Build(opts...)
	if err != nil {
		return nil, err
	}

	if customCore != nil {
		logger = zap.New(customCore, opts...)
	}

	if cfg != nil && len(cfg.Namespace) > 0 {
		logger = logger.With(zap.Namespace(cfg.Namespace))
	}

	if component != "" {
		logger = logger.With(zap.String("component", component))
	}

	return logger, nil
}

// NewBootstrap creates a simple production logger for bootstrap operations
// This is used during configuration loading when we don't yet have config
func NewBootstrap() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.DisableStacktrace = true
	logger, err := cfg.Build()
	if err != nil {
		// Fall back to a no-op logger if we can't create a production logger
		return zap.NewNop()
	}
	return logger
}

// NewDevelopment creates a development logger with sensible defaults
// Useful for local development and debugging
func NewDevelopment() (*zap.Logger, error) {
	return zap.NewDevelopment(zap.AddStacktrace(zap.ErrorLevel))
}

// NewProduction creates a production logger with sensible defaults
// Useful when you need a logger but don't have configuration
func NewProduction() (*zap.Logger, error) {
	return zap.NewProduction()
}

// NewNop creates a no-op logger that discards all logs
// Useful for testing when you don't care about log output
func NewNop() *zap.Logger {
	return zap.NewNop()
}

// Must is a helper that wraps a logger creation function and panics on error
// Useful for cases where you know the logger creation cannot fail
func Must(logger *zap.Logger, err error) *zap.Logger {
	if err != nil {
		panic(err)
	}
	return logger
}

