package config

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/logger"
)

type Server struct {
	Listener             Listener       `yaml:"listener"`
	Timeouts             Timeouts       `yaml:"timeouts"`
	Logger               *logger.Config `yaml:"logger"`
	AccessLog            *AccessLog     `yaml:"access_log"`
	Stats                *Stats         `yaml:"stats"`
	EnablePprof          bool           `yaml:"enable_pprof"`
	MaxResponseSizeBytes int            `yaml:"max_response_size_bytes"`
}

func (s *Server) SetDefaults() {
	if s == nil {
		return
	}
	s.Listener.SetDefaults()

	if s.Logger == nil {
		s.Logger = &logger.Config{Level: zap.ErrorLevel}
	}

	if s.Stats == nil {
		s.Stats = &Stats{
			FlushInterval: time.Second,
			Prefix:        "admiral",
			ReporterType:  ReporterTypeNull,
		}
	}
}

func (s *Server) Validate() error {
	if s == nil {
		return nil
	}
	if s.Stats != nil {
		if err := s.Stats.ReporterType.Validate(); err != nil {
			return fmt.Errorf("invalid stats.reporter_type: %w", err)
		}
	}

	return nil
}

type Listener struct {
	Address string `yaml:"address" validate:"ip"`
	Port    int    `yaml:"port" validate:"required,min=1,max=65535"`
}

func (l *Listener) SetDefaults() {
	if l.Address == "" {
		l.Address = "0.0.0.0"
	}
	if l.Port == 0 {
		l.Port = 8080
	}
}

type Timeouts struct {
	Default   time.Duration   `yaml:"default"`
	Overrides []TimeoutsEntry `yaml:"overrides"`
}

type TimeoutsEntry struct {
	Service string        `yaml:"service"`
	Method  string        `yaml:"method"`
	Timeout time.Duration `yaml:"timeout"`
}

type AccessLog struct {
	StatusCodeFilters []uint32 `yaml:"status_code_filters"`
}

type Stats struct {
	FlushInterval  time.Duration   `yaml:"flush_interval"`
	GoRuntimeStats *GoRuntimeStats `yaml:"go_runtime_stats"`
	Prefix         string          `yaml:"prefix"`
	ReporterType   ReporterType    `yaml:"reporter_type"`
}

type GoRuntimeStats struct {
	CollectionInterval *time.Duration `yaml:"collection_interval" validate:"required,min=1s"`
}

type ReporterType string

const (
	ReporterTypeNull       ReporterType = "null"
	ReporterTypeLog        ReporterType = "log"
	ReporterTypePrometheus ReporterType = "prometheus"
)

func (r ReporterType) Validate() error {
	switch r {
	case ReporterTypeNull, ReporterTypeLog, ReporterTypePrometheus:
		return nil
	default:
		return fmt.Errorf("invalid reporter type: %q (valid: null, log, prometheus)", r)
	}
}
