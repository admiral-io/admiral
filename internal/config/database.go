package config

import (
	"fmt"
	"strings"
	"time"
)

type Database struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	DatabaseName      string        `yaml:"database_name"`
	User              string        `yaml:"user"`
	Password          string        `yaml:"password"`
	SSLMode           SSLMode       `yaml:"ssl_mode"`
	MaxOpenConns      int           `yaml:"max_open_conns"`
	MaxIdleConns      int           `yaml:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime   time.Duration `yaml:"conn_max_idle_time"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`
}

func (d *Database) SetDefaults() {
	if d == nil {
		return
	}

	if d.Port == 0 {
		d.Port = 5432
	}

	if d.DatabaseName == "" {
		d.DatabaseName = "admiral"
	}

	if d.SSLMode == SSLModeUnspecified {
		d.SSLMode = SSLModeRequire
	}

	if d.MaxOpenConns == 0 {
		d.MaxOpenConns = 100
	}

	if d.MaxIdleConns == 0 {
		d.MaxIdleConns = 10
	}

	if d.ConnMaxLifetime == 0 {
		d.ConnMaxLifetime = 30 * time.Minute
	}

	if d.ConnMaxIdleTime == 0 {
		d.ConnMaxIdleTime = 5 * time.Minute
	}

	if d.ConnectionTimeout == 0 {
		d.ConnectionTimeout = 5 * time.Second
	}
}

func (d *Database) Validate() error {
	if d == nil {
		return nil
	}

	if d.Host == "" {
		return fmt.Errorf("host is required")
	}

	if d.User == "" {
		return fmt.Errorf("user is required")
	}

	if d.Password == "" {
		return fmt.Errorf("password is required")
	}

	return d.SSLMode.Validate()
}

type SSLMode int

const (
	SSLModeUnspecified SSLMode = iota
	SSLModeDisable
	SSLModeAllow
	SSLModePrefer
	SSLModeRequire
	SSLModeVerifyCA
	SSLModeVerifyFull
)

var sslModeName = map[SSLMode]string{
	SSLModeUnspecified: "unspecified",
	SSLModeDisable:     "disable",
	SSLModeAllow:       "allow",
	SSLModePrefer:      "prefer",
	SSLModeRequire:     "require",
	SSLModeVerifyCA:    "verify_ca",
	SSLModeVerifyFull:  "verify_full",
}

var sslModeValue = map[string]SSLMode{
	"unspecified": SSLModeUnspecified,
	"disable":     SSLModeDisable,
	"allow":       SSLModeAllow,
	"prefer":      SSLModePrefer,
	"require":     SSLModeRequire,
	"verify_ca":   SSLModeVerifyCA,
	"verify_full": SSLModeVerifyFull,
}

func (s *SSLMode) String() string {
	if s == nil {
		return sslModeName[SSLModeUnspecified]
	}

	if name, ok := sslModeName[*s]; ok {
		return name
	}

	return sslModeName[SSLModeUnspecified]
}

func (s *SSLMode) MarshalYAML() (any, error) {
	return s.String(), nil
}

func (s *SSLMode) UnmarshalYAML(unmarshal func(any) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}

	str = strings.ToLower(str)
	if val, ok := sslModeValue[str]; ok {
		*s = val
		return nil
	}

	return fmt.Errorf("invalid SSLMode: %q", str)
}

func (s *SSLMode) Validate() error {
	if s == nil {
		return fmt.Errorf("SSLMode is nil")
	}

	if _, ok := sslModeName[*s]; !ok {
		return fmt.Errorf("invalid SSLMode: %d", *s)
	}

	return nil
}
