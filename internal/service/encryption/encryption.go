package encryption

import (
	"fmt"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/crypto"
	"go.admiral.io/admiral/internal/service"
)

const Name = "service.encryption"

// Service provides encryption and decryption for sensitive data.
type Service interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(envelopeJSON []byte) ([]byte, error)
	DecryptAny(envelopeJSON []byte) ([]byte, error)
	HasOldKey() bool
}

func New(cfg *config.Config, _ *zap.Logger, _ tally.Scope) (service.Service, error) {
	if cfg.Services.Encryption == nil {
		return nil, fmt.Errorf("encryption configuration is required")
	}

	enc, err := crypto.NewEncryptor(cfg.Services.Encryption.ActiveKey, cfg.Services.Encryption.OldKey)
	if err != nil {
		return nil, fmt.Errorf("encryption: %w", err)
	}

	return enc, nil
}
