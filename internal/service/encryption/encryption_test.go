package encryption

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.admiral.io/admiral/internal/config"
)

func generateKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(key)
}

func testConfig(activeKey, oldKey string) *config.Config {
	return &config.Config{
		Services: config.Services{
			Encryption: &config.Encryption{
				ActiveKey: activeKey,
				OldKey:    oldKey,
			},
		},
	}
}

func TestName(t *testing.T) {
	assert.Equal(t, "service.encryption", Name)
}

func TestNew(t *testing.T) {
	t.Run("nil encryption config", func(t *testing.T) {
		cfg := &config.Config{Services: config.Services{}}
		_, err := New(cfg, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "encryption configuration is required")
	})

	t.Run("empty active key", func(t *testing.T) {
		cfg := testConfig("", "")
		_, err := New(cfg, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "encryption")
	})

	t.Run("invalid active key", func(t *testing.T) {
		cfg := testConfig("not-valid-base64!!!", "")
		_, err := New(cfg, nil, nil)
		require.Error(t, err)
	})

	t.Run("wrong length key", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString([]byte("too-short"))
		cfg := testConfig(short, "")
		_, err := New(cfg, nil, nil)
		require.Error(t, err)
	})

	t.Run("valid active key only", func(t *testing.T) {
		cfg := testConfig(generateKey(t), "")
		svc, err := New(cfg, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, svc)

		enc, ok := svc.(Service)
		require.True(t, ok)
		assert.False(t, enc.HasOldKey())
	})

	t.Run("valid active and old keys", func(t *testing.T) {
		cfg := testConfig(generateKey(t), generateKey(t))
		svc, err := New(cfg, nil, nil)
		require.NoError(t, err)

		enc, ok := svc.(Service)
		require.True(t, ok)
		assert.True(t, enc.HasOldKey())
	})
}

func TestServiceInterface(t *testing.T) {
	cfg := testConfig(generateKey(t), "")
	svc, err := New(cfg, nil, nil)
	require.NoError(t, err)

	enc, ok := svc.(Service)
	require.True(t, ok, "returned service must satisfy encryption.Service")

	t.Run("encrypt and decrypt round trip", func(t *testing.T) {
		plaintext := []byte(`{"token":"secret-value"}`)
		encrypted, err := enc.Encrypt(plaintext)
		require.NoError(t, err)
		assert.NotEqual(t, plaintext, encrypted)

		decrypted, err := enc.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("decrypt any", func(t *testing.T) {
		plaintext := []byte(`sensitive data`)
		encrypted, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		decrypted, err := enc.DecryptAny(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})
}

func TestServiceKeyRotation(t *testing.T) {
	oldKey := generateKey(t)
	newKey := generateKey(t)

	// Encrypt with old key as active.
	cfg1 := testConfig(oldKey, "")
	svc1, err := New(cfg1, nil, nil)
	require.NoError(t, err)
	enc1 := svc1.(Service)

	plaintext := []byte(`{"secret":"rotate-me"}`)
	encrypted, err := enc1.Encrypt(plaintext)
	require.NoError(t, err)

	// Create new service with rotated keys.
	cfg2 := testConfig(newKey, oldKey)
	svc2, err := New(cfg2, nil, nil)
	require.NoError(t, err)
	enc2 := svc2.(Service)

	// DecryptAny should handle data encrypted with the now-old key.
	decrypted, err := enc2.DecryptAny(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Re-encrypt with new key.
	reEncrypted, err := enc2.Encrypt(decrypted)
	require.NoError(t, err)

	// Standard Decrypt should work with the new key.
	decrypted2, err := enc2.Decrypt(reEncrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)
}
