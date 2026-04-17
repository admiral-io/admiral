package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Configurable interface {
	SetDefaults()
	Validate() error
}

type Config struct {
	Server   *Server  `yaml:"server"`
	Services Services `yaml:"services"`
}

type Services struct {
	Authn         *Authn         `yaml:"authn"`
	Database      *Database      `yaml:"database"`
	Session       *Session       `yaml:"session"`
	ObjectStorage *ObjectStorage `yaml:"object_storage"`
	//Temporal      *Temporal      `yaml:"temporal"`
}

func Build(file string, envFiles []string, debug bool) (*Config, error) {
	if err := loadEnv(envFiles); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	cfg, err := parseConfig(file, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return cfg, nil
}

func loadEnv(envFiles []string) error {
	// Order is important as godotenv will NOT overwrite existing environment variables.
	for _, filename := range envFiles {
		p, err := filepath.Abs(filename)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", filename, err)
		}

		if err := godotenv.Load(p); err != nil {
			// Non-fatal: continue loading other files even if one fails.
			continue
		}
	}

	return nil
}

func parseConfig(file string, debug bool) (*Config, error) {
	contents, err := os.ReadFile(file) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Replace environment variables in the configuration content.
	expandedContents := []byte(os.ExpandEnv(string(contents)))

	cfg := &Config{}
	if err = yaml.Unmarshal(expandedContents, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// If a debug flag is set, print the configuration and exit.
	if debug {
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal configuration to JSON: %w", err)
		}
		fmt.Print(string(b))
		os.Exit(0)
	}

	cfg = setDefaults(cfg)

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("struct tag validation failed: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

func setDefaults(cfg *Config) *Config {
	if cfg.Server == nil {
		cfg.Server = &Server{}
	}
	if cfg.Services.Session == nil {
		cfg.Services.Session = &Session{}
	}

	configs := []Configurable{
		cfg.Server,
		cfg.Services.Authn,
		cfg.Services.Database,
		cfg.Services.Session,
		cfg.Services.ObjectStorage,
		//cfg.Services.Temporal,
	}

	for _, c := range configs {
		if c != nil {
			c.SetDefaults()
		}
	}

	return cfg
}

func (c *Config) validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	// Define all configs with their requirements
	configs := []struct {
		config   Configurable
		name     string
		required bool
	}{
		// Optional configs
		{c.Server, "server", false},
		{c.Services.Authn, "services.authn", false},
		{c.Services.Database, "services.database", true},
		{c.Services.Session, "services.session", false},
		{c.Services.ObjectStorage, "services.object_storage", true},
		//{c.Services.Temporal, "services.temporal", true},
	}

	// Validate all configs
	for _, cfg := range configs {
		if err := validateConfigItem(cfg.config, cfg.name, cfg.required); err != nil {
			return err
		}
	}

	return nil
}

func validateConfigItem(c Configurable, name string, required bool) error {
	// Check if the interface contains a nil value
	if c == nil || reflect.ValueOf(c).IsNil() {
		if required {
			return fmt.Errorf("%s config is nil", name)
		}
		return nil // Optional and nil are OK
	}
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid %s config: %w", name, err)
	}
	return nil
}
