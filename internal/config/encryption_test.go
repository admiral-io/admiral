package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestEncryptionValidate(t *testing.T) {
	tests := []struct {
		name    string
		enc     *Encryption
		wantErr string
	}{
		{name: "nil", enc: nil},
		{name: "valid active only", enc: &Encryption{ActiveKey: validKey()}},
		{name: "valid active and old", enc: &Encryption{ActiveKey: validKey(), OldKey: validKey()}},
		{name: "empty active key", enc: &Encryption{}, wantErr: "active_key is required"},
		{name: "invalid base64 active", enc: &Encryption{ActiveKey: "not-base64!!!"}, wantErr: "invalid base64"},
		{name: "wrong length active", enc: &Encryption{ActiveKey: base64.StdEncoding.EncodeToString([]byte("short"))}, wantErr: "key must be 32 bytes"},
		{name: "invalid old key", enc: &Encryption{ActiveKey: validKey(), OldKey: "bad"}, wantErr: "old_key"},
		{name: "empty old key is fine", enc: &Encryption{ActiveKey: validKey(), OldKey: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.enc.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEncryptionSetDefaults(t *testing.T) {
	e := &Encryption{}
	e.SetDefaults()
	// SetDefaults is a no-op; just verify no panic.
	assert.Empty(t, e.ActiveKey)
}

func TestEncryptionNewEncryptor(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		e := &Encryption{ActiveKey: validKey()}
		enc, err := e.NewEncryptor()
		require.NoError(t, err)
		assert.NotNil(t, enc)
	})

	t.Run("invalid key", func(t *testing.T) {
		e := &Encryption{ActiveKey: "bad"}
		_, err := e.NewEncryptor()
		assert.Error(t, err)
	})
}
