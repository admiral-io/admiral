package server

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/crypto"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service/database"
)

type rotateKeysCmd struct {
	cmd  *cobra.Command
	opts rotateKeysOpts
}

type rotateKeysOpts struct {
	status bool
}

type keyStatus struct {
	Active      int
	Old         int
	Unencrypted int
	Total       int
}

func newRotateKeysCmd(globals *globalOpts) *rotateKeysCmd {
	rc := &rotateKeysCmd{}

	cmd := &cobra.Command{
		Use:   "rotate-keys",
		Short: "Re-encrypt credentials with the active encryption key",
		Long: `Re-encrypt all credential auth_config values using the active encryption key.

Run this after rotating encryption keys to migrate all credentials from the
old key to the new active key. Once complete, the old key can be safely
removed from the configuration.

Use --status to see how many credentials use each key without making changes.`,
		Example: `  # Check current encryption status
  admiral-server rotate-keys --status

  # Re-encrypt all credentials with the active key
  admiral-server rotate-keys

  # Use custom configuration
  admiral-server --config /path/to/config.yaml rotate-keys`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.FatalLevel + 1))
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() { _ = logger.Sync() }()

			cfg, err := config.Build(globals.configFile, globals.envVarFiles, globals.debug)
			if err != nil {
				return err
			}

			if cfg.Services.Encryption == nil {
				return fmt.Errorf("services.encryption is not configured")
			}

			enc, err := cfg.Services.Encryption.NewEncryptor()
			if err != nil {
				return fmt.Errorf("encryption config: %w", err)
			}

			db, err := openGormDB(cfg)
			if err != nil {
				return err
			}
			defer closeGormDB(db)

			if rc.opts.status {
				return runStatus(logger, db, enc)
			}

			return runRotate(logger, db, enc)
		},
	}

	cmd.Flags().BoolVar(&rc.opts.status, "status", false, "show encryption status without making changes")
	rc.cmd = cmd
	return rc
}

func classifyCredentials(db *gorm.DB) (*keyStatus, []model.Credential, error) {
	var creds []model.Credential
	if err := db.Find(&creds).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	status := &keyStatus{Total: len(creds)}
	for _, c := range creds {
		raw := c.AuthConfig.Raw()
		if raw == nil || !crypto.IsEnvelope(raw) {
			status.Unencrypted++
			continue
		}
		if crypto.NeedsRotation(raw) {
			status.Old++
		} else {
			status.Active++
		}
	}

	return status, creds, nil
}

func runStatus(log *zap.Logger, db *gorm.DB, _ *crypto.Encryptor) error {
	status, _, err := classifyCredentials(db)
	if err != nil {
		return err
	}

	log.Info("Encryption status",
		zap.Int("total", status.Total),
		zap.Int("active_key", status.Active),
		zap.Int("old_key", status.Old),
		zap.Int("unencrypted", status.Unencrypted),
	)

	if status.Old == 0 {
		log.Info("All credentials are encrypted with the active key")
	} else {
		log.Info("Credentials need rotation",
			zap.Int("count", status.Old),
		)
	}

	return nil
}

func runRotate(log *zap.Logger, db *gorm.DB, enc *crypto.Encryptor) error {
	status, creds, err := classifyCredentials(db)
	if err != nil {
		return err
	}

	if status.Old == 0 {
		log.Info("All credentials are already encrypted with the active key, nothing to do",
			zap.Int("total", status.Total),
		)
		return nil
	}

	log.Info("Starting key rotation",
		zap.Int("total", status.Total),
		zap.Int("to_rotate", status.Old),
	)

	rotated := 0
	for _, cred := range creds {
		raw := cred.AuthConfig.Raw()
		if raw == nil || !crypto.IsEnvelope(raw) || !crypto.NeedsRotation(raw) {
			continue
		}

		plaintext, err := enc.DecryptAny(raw)
		if err != nil {
			return fmt.Errorf("credential %s: failed to decrypt: %w", cred.Id, err)
		}

		envelope, err := enc.Encrypt(plaintext)
		if err != nil {
			return fmt.Errorf("credential %s: failed to encrypt: %w", cred.Id, err)
		}

		if err := db.Model(&model.Credential{}).
			Where("id = ?", cred.Id).
			Update("auth_config", string(envelope)).Error; err != nil {
			return fmt.Errorf("credential %s: failed to update: %w", cred.Id, err)
		}

		rotated++
		log.Debug("Rotated credential", zap.String("id", cred.Id.String()), zap.String("name", cred.Name))
	}

	log.Info("Key rotation complete",
		zap.Int("rotated", rotated),
		zap.Int("skipped", status.Total-rotated),
	)

	return nil
}

func openGormDB(cfg *config.Config) (*gorm.DB, error) {
	connStr, err := database.ConnString(cfg.Services.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to build connection string: %w", err)
	}

	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger:                 gormlogger.Discard,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to open GORM connection: %w", err)
	}

	return db, nil
}

func closeGormDB(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
