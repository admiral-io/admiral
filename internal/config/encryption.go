package config

import (
	"encoding/base64"
	"fmt"

	"go.admiral.io/admiral/internal/crypto"
)

const encryptionKeyLen = 32 // AES-256

// Encryption configures app-layer encryption for sensitive data (credential
// auth_config). Keys are base64-encoded 32-byte values.
type Encryption struct {
	// ActiveKey encrypts all new data. Required once encryption is enabled.
	ActiveKey string `yaml:"active_key"`

	// OldKey decrypts data written before a key rotation. Optional.
	// Present only during the rotation window; remove after running rotate-keys.
	OldKey string `yaml:"old_key"`
}

func (e *Encryption) SetDefaults() {}

func (e *Encryption) Validate() error {
	if e == nil {
		return nil
	}

	if e.ActiveKey == "" {
		return fmt.Errorf("encryption.active_key is required")
	}

	if err := validateKeyEncoding(e.ActiveKey, "active_key"); err != nil {
		return err
	}

	if e.OldKey != "" {
		if err := validateKeyEncoding(e.OldKey, "old_key"); err != nil {
			return err
		}
	}

	return nil
}

// NewEncryptor builds a crypto.Encryptor from this config.
func (e *Encryption) NewEncryptor() (*crypto.Encryptor, error) {
	return crypto.NewEncryptor(e.ActiveKey, e.OldKey)
}

func validateKeyEncoding(b64, name string) error {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("encryption.%s: invalid base64: %w", name, err)
	}
	if len(raw) != encryptionKeyLen {
		return fmt.Errorf("encryption.%s: key must be %d bytes, got %d", name, encryptionKeyLen, len(raw))
	}
	return nil
}
