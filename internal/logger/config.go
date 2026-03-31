package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config represents logger configuration.
type Config struct {
	Level     zapcore.Level `yaml:"level"`
	Pretty    bool          `yaml:"pretty"`
	Namespace string        `yaml:"namespace"`
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type raw Config
	r := raw{Level: zap.ErrorLevel}
	if err := unmarshal(&r); err != nil {
		return err
	}
	*c = Config(r)
	return nil
}

// NewConfig creates a logger config with sensible defaults.
func NewConfig() *Config {
	return &Config{
		Level:     zapcore.InfoLevel,
		Pretty:    false,
		Namespace: "",
	}
}

// WithLevel sets the log level.
func (c *Config) WithLevel(level zapcore.Level) *Config {
	c.Level = level
	return c
}

// WithPretty enables/disables pretty printing.
func (c *Config) WithPretty(pretty bool) *Config {
	c.Pretty = pretty
	return c
}

// WithNamespace sets the namespace for field grouping.
func (c *Config) WithNamespace(namespace string) *Config {
	c.Namespace = namespace
	return c
}

// Development returns a config suitable for development.
func Development() *Config {
	return &Config{
		Level:     zapcore.DebugLevel,
		Pretty:    true,
		Namespace: "",
	}
}

// Production returns a config suitable for production.
func Production() *Config {
	return &Config{
		Level:     zapcore.InfoLevel,
		Pretty:    false,
		Namespace: "",
	}
}
