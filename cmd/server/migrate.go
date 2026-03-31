package server

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/service/database"
)

type migrateCmd struct {
	cmd  *cobra.Command
	opts migrateOpts
}

type migrateOpts struct {
	force bool
	down  bool
	reset bool
}

type migrator struct {
	log    *zap.Logger
	config *config.Config
	force  bool
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

func newMigrateCmd(globals *globalOpts) *migrateCmd {
	mc := &migrateCmd{}

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database schema migrations",
		Long: `Apply, rollback, or reset database schema migrations for Admiral.

By default, all pending migrations are applied. Use --down to rollback the
last migration, or --reset to clear a dirty migration state.`,
		Example: `  # Apply all pending migrations
  admiral migrate

  # Apply migrations without confirmation prompts
  admiral migrate --force

  # Rollback the last migration
  admiral migrate --down

  # Reset dirty migration state
  admiral migrate --reset

  # Use custom configuration
  admiral --config /path/to/config.yaml migrate`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.FatalLevel + 1))
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() { _ = logger.Sync() }()

			cfg := config.Build(globals.configFile, globals.envVarFiles, globals.debug)
			m := &migrator{
				log:    logger,
				config: cfg,
				force:  mc.opts.force,
			}

			switch {
			case mc.opts.reset:
				return m.Reset()
			case mc.opts.down:
				return m.Down()
			default:
				return m.Up()
			}
		},
	}

	cmd.Flags().BoolVarP(&mc.opts.force, "force", "f", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&mc.opts.down, "down", false, "rollback one migration")
	cmd.Flags().BoolVar(&mc.opts.reset, "reset", false, "reset dirty migration state")
	cmd.MarkFlagsMutuallyExclusive("down", "reset")

	mc.cmd = cmd
	return mc
}

func (m *migrator) setupSqlClient() (*sql.DB, error) {
	connStr, err := database.ConnString(m.config.Services.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to build connection string: %w", err)
	}

	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Migrations only need a single connection.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	return sqlDB, nil
}

func (m *migrator) setupSqlMigrator() (*migrate.Migrate, error) {
	sqlDB, err := m.setupSqlClient()
	if err != nil {
		return nil, err
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	dbDriver, err := postgres.WithInstance(sqlDB, &postgres.Config{
		MigrationsTable: postgres.DefaultMigrationsTable,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	srcDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to create source driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", srcDriver, "postgres", dbDriver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	migrator.Log = &migrateLogger{logger: m.log.Sugar()}
	return migrator, nil
}

func (m *migrator) closeMigrate(mg *migrate.Migrate) {
	srcErr, dbErr := mg.Close()
	if srcErr != nil {
		m.log.Error("failed to close migration source", zap.Error(srcErr))
	}
	if dbErr != nil {
		m.log.Error("failed to close migration database", zap.Error(dbErr))
	}
}

func (m *migrator) hostInfo() string {
	return fmt.Sprintf("%s@%s:%d",
		m.config.Services.Database.User,
		m.config.Services.Database.Host,
		m.config.Services.Database.Port,
	)
}

func (m *migrator) confirmWithUser(msg string) error {
	m.log.Info("Using database", zap.String("host", m.hostInfo()))
	if !m.force {
		m.log.Warn(msg)
		fmt.Printf("\n*** Continue with migration? [y/N] ")
		var answer string
		if _, err := fmt.Scanln(&answer); err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("stdin is not interactive; use '--force' to skip confirmation")
			}
			if !strings.Contains(err.Error(), "unexpected newline") {
				return fmt.Errorf("failed to read user input: %w", err)
			}
		}
		if strings.ToLower(answer) != "y" {
			return errors.New("migration aborted; enter 'y' to continue or use '-f' flag")
		}
		fmt.Println()
	}
	return nil
}

func (m *migrator) Up() error {
	migrator, err := m.setupSqlMigrator()
	if err != nil {
		return err
	}
	defer m.closeMigrate(migrator)

	msg := "Migration may cause data loss; verify the database details above."
	if err := m.confirmWithUser(msg); err != nil {
		return err
	}

	m.log.Info("Applying up migrations")
	if err := migrator.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.log.Info("No pending migrations")
			return nil
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	m.log.Info("Migrations applied successfully")
	return nil
}

func (m *migrator) Down() error {
	migrator, err := m.setupSqlMigrator()
	if err != nil {
		return err
	}
	defer m.closeMigrate(migrator)

	version, _, err := migrator.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return errors.New("no migrations have been applied; nothing to rollback")
		}
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	msg := fmt.Sprintf(
		"Rolling back migration version %d; this may cause data loss.",
		version,
	)
	if err := m.confirmWithUser(msg); err != nil {
		return err
	}

	m.log.Info("Applying down migration")
	if err := migrator.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply down migration: %w", err)
	}
	m.log.Info("Down migration applied successfully")
	return nil
}

func (m *migrator) Reset() error {
	migrator, err := m.setupSqlMigrator()
	if err != nil {
		return err
	}
	defer m.closeMigrate(migrator)

	version, dirty, err := migrator.Version()
	if err != nil {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if !dirty {
		m.log.Info("Schema is not dirty, nothing to reset")
		return nil
	}

	msg := fmt.Sprintf("Schema is dirty at version %d. Resetting will allow future migrations but may cause inconsistencies.", version)
	if err := m.confirmWithUser(msg); err != nil {
		return err
	}

	if err := migrator.Force(int(version)); err != nil { //nolint:gosec
		return fmt.Errorf("failed to force migration version: %w", err)
	}

	m.log.Info("Migration state reset; dirty flag cleared")
	return nil
}

type migrateLogger struct {
	logger *zap.SugaredLogger
}

func (m *migrateLogger) Printf(format string, v ...interface{}) {
	m.logger.Infof(strings.TrimRight(format, "\n"), v...)
}

func (m *migrateLogger) Verbose() bool {
	return true
}
