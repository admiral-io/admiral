package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Configurable interface {
	SetDefaults()
	Validate() error
}

type Config struct {
	Server *Server `yaml:"server"`
	//Endpoints *Endpoints `yaml:"endpoints"`
	Services Services `yaml:"services"`
}

type Services struct {
	//Authn         *Authn         `yaml:"authn"`
	Database *Database `yaml:"database"`
	//ObjectStorage *ObjectStorage `yaml:"object_storage"`
	//Session       *Session       `yaml:"session"`
	//Temporal      *Temporal      `yaml:"temporal"`
}

func Build(file string, envFiles []string, debug bool) *Config {
	tmpLogger := newTmpLogger()

	// Load environment variables from .env files.
	if err := loadEnv(envFiles); err != nil {
		tmpLogger.Fatal("Failed to load environment variables", zap.Error(err))
	}

	// Parse the configuration file.
	cfg, err := parseConfig(file, debug)
	if err != nil {
		tmpLogger.Fatal("Failed to load environment variables", zap.Error(err))
	}

	return cfg
}

func loadEnv(envFiles []string) error {
	// Order is important as godotenv will NOT overwrite existing environment variables.
	for _, filename := range envFiles {
		// Use a temporary logger to parse the environment files
		tmpLogger := newTmpLogger().With(zap.String("file", filename))

		p, err := filepath.Abs(filename)
		if err != nil {
			tmpLogger.Error("failed to get absolute path for .env file", zap.Error(err))
			return err
		}

		// Load the environment variables from the .env file.
		if err := godotenv.Load(p); err != nil {
			// Log a warning if the .env file is not found or cannot be loaded.
			tmpLogger.Warn("Could not load .env file", zap.Error(err))
			continue // Continue loading other files even if one fails.
		}
	}

	return nil
}

func parseConfig(file string, debug bool) (*Config, error) {
	// Use a temporary logger to parse the configuration and output.
	tmpLogger := newTmpLogger().With(zap.String("file", file))

	// Read the configuration file.
	contents, err := os.ReadFile(file) //nolint:gosec
	if err != nil {
		tmpLogger.Error("failed to read configuration file", zap.Error(err))
		return nil, err
	}

	// Replace environment variables in the configuration content.
	expandedContents := []byte(os.ExpandEnv(string(contents)))

	// Unmarshal the YAML configuration into the config struct.
	cfg := &Config{}
	if err = yaml.Unmarshal(expandedContents, cfg); err != nil {
		tmpLogger.Error("failed to parse configuration file", zap.Error(err))
		return nil, err
	}

	// If a debug flag is set, print the configuration and exit.
	if debug {
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			tmpLogger.Fatal("failed to cast configuration file to json", zap.Error(err))
		}
		fmt.Print(string(b))
		os.Exit(0)
	}

	// Set default values in the configuration.
	cfg = setDefaults(cfg)

	// Validate the configuration
	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		tmpLogger.Fatal("struct tag validation failed", zap.Error(err))
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		tmpLogger.Fatal("custom configuration validation failed", zap.Error(err))
		return nil, err
	}

	return cfg, nil
}

func setDefaults(cfg *Config) *Config {
	if cfg.Server == nil {
		cfg.Server = &Server{}
	}
	//if cfg.Services.Session == nil {
	//	cfg.Services.Session = &Session{}
	//}

	configs := []Configurable{
		cfg.Server,
		//cfg.Endpoints,
		cfg.Services.Database,
		//cfg.Services.Temporal,
		//cfg.Services.ObjectStorage,
		//cfg.Services.Authn,
		//cfg.Services.Session,
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
		//{c.Endpoints, "endpoints", false},
		//{c.Services.Authn, "services.authn", false},
		//{c.Services.Session, "services.session", false},
		// Required configs
		{c.Services.Database, "services.database", true},
		//{c.Services.ObjectStorage, "services.object_storage", true},
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
