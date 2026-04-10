package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"moul.io/zapgorm2"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/service"
)

const Name = "service.database"

type srv struct {
	sqlDB  *sql.DB
	gormDB *gorm.DB
	logger *zap.Logger
	scope  tally.Scope
}

type Service interface {
	DB() *sql.DB
	GormDB() *gorm.DB
}

func (s *srv) DB() *sql.DB {
	return s.sqlDB
}

func (s *srv) GormDB() *gorm.DB {
	return s.gormDB
}

func New(cfg *config.Config, logger *zap.Logger, scope tally.Scope) (service.Service, error) {
	dbCfg := cfg.Services.Database

	connection, err := ConnString(dbCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := sql.Open("postgres", connection)
	if err != nil {
		return nil, err
	}

	if dbCfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(dbCfg.MaxOpenConns)
	} else {
		sqlDB.SetMaxOpenConns(100)
	}

	if dbCfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(dbCfg.MaxIdleConns)
	} else {
		sqlDB.SetMaxIdleConns(10)
	}

	if dbCfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	} else {
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
	}

	if dbCfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(dbCfg.ConnMaxIdleTime)
	} else {
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	}

	// Test connection
	timeout := dbCfg.ConnectionTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	zapGormLogger := zapgorm2.New(logger)
	zapGormLogger.LogLevel = gormlogger.Silent

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger:                 zapGormLogger,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to open GORM connection: %w", err)
	}

	logger.Info("database connected",
		zap.String("host", cfg.Services.Database.Host),
		zap.Int("port", cfg.Services.Database.Port),
		zap.String("dbname", cfg.Services.Database.DatabaseName))

	scope.Counter("database_connection_success").Inc(1)

	return &srv{sqlDB: sqlDB, gormDB: gormDB, logger: logger, scope: scope}, nil
}

func ConnString(cfg *config.Database) (string, error) {
	if cfg == nil {
		return "", errors.New("no connection information")
	}

	if cfg.Host == "" || strings.ContainsAny(cfg.Host, " \t\n\r'\"\\=") {
		return "", fmt.Errorf("invalid host: %s", cfg.Host)
	}
	if cfg.DatabaseName == "" {
		return "", errors.New("database name is required")
	}

	connection := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s connect_timeout=10 application_name=admiral",
		cfg.Host, cfg.Port, cfg.DatabaseName, cfg.User, cfg.Password,
	)

	validSSLModes := map[config.SSLMode]string{
		config.SSLModeUnspecified: "disable",
		config.SSLModeDisable:     "disable",
		config.SSLModeAllow:       "allow",
		config.SSLModePrefer:      "prefer",
		config.SSLModeRequire:     "require",
		config.SSLModeVerifyCA:    "verify-ca",
		config.SSLModeVerifyFull:  "verify-full",
	}
	mode, ok := validSSLModes[cfg.SSLMode]
	if !ok {
		return "", fmt.Errorf("invalid SSLMode: %v", cfg.SSLMode)
	}
	connection += fmt.Sprintf(" sslmode=%s", mode)

	return connection, nil
}
