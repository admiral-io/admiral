package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(key)
}

func TestNewEncryptor(t *testing.T) {
	t.Run("valid active key only", func(t *testing.T) {
		enc, err := NewEncryptor(generateKey(t), "")
		require.NoError(t, err)
		assert.NotNil(t, enc)
		assert.False(t, enc.HasOldKey())
	})

	t.Run("valid active and old keys", func(t *testing.T) {
		enc, err := NewEncryptor(generateKey(t), generateKey(t))
		require.NoError(t, err)
		assert.NotNil(t, enc)
		assert.True(t, enc.HasOldKey())
	})

	t.Run("empty active key", func(t *testing.T) {
		_, err := NewEncryptor("", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "active encryption key is required")
	})

	t.Run("invalid base64 active key", func(t *testing.T) {
		_, err := NewEncryptor("not-valid-base64!!!", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid active key")
	})

	t.Run("wrong length active key", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString([]byte("too-short"))
		_, err := NewEncryptor(short, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key must be 32 bytes")
	})

	t.Run("invalid old key", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString([]byte("too-short"))
		_, err := NewEncryptor(generateKey(t), short)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid old key")
	})
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	plaintext := []byte(`{"type":"bearer_token","bearer_token":{"token":"secret123"}}`)

	encrypted, err := enc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	// Verify it's valid JSON envelope
	assert.True(t, IsEnvelope(encrypted))

	decrypted, err := enc.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	plaintext := []byte("same message")

	a, err := enc.Encrypt(plaintext)
	require.NoError(t, err)

	b, err := enc.Encrypt(plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, a, b, "each encryption should produce different output due to random nonce")
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	_, err = enc.Encrypt(nil)
	assert.Error(t, err)

	_, err = enc.Encrypt([]byte{})
	assert.Error(t, err)
}

func TestKeyRotation(t *testing.T) {
	oldKey := generateKey(t)
	newKey := generateKey(t)

	// Phase 1: encrypt with old key (it was the active key before rotation)
	encBefore, err := NewEncryptor(oldKey, "")
	require.NoError(t, err)

	plaintext := []byte(`{"type":"ssh_key","ssh_key":{"private_key":"-----BEGIN..."}}`)
	encrypted, err := encBefore.Encrypt(plaintext)
	require.NoError(t, err)

	// The envelope says "active" because oldKey was the active key at write time.
	// After rotation, we need to re-tag it as "old" so the new encryptor can find it.
	// In practice, rotate-keys handles this: decrypt with old, re-encrypt with new.
	// But during the mixed-read window, the store layer needs to try both keys.

	// Phase 2: new encryptor with new active + old key
	encAfter, err := NewEncryptor(newKey, oldKey)
	require.NoError(t, err)

	// The encrypted data still says key="active" but was encrypted with what is
	// now the old key. Decrypt should try active first, fail, then try old.
	// However, our current design uses the slot label, not trial decryption.
	// So for the migration window, we need DecryptAny which tries both.

	decrypted, err := encAfter.DecryptAny(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Phase 3: re-encrypt with new active key
	reEncrypted, err := encAfter.Encrypt(decrypted)
	require.NoError(t, err)

	// Now decrypt should work with just the active key
	decrypted2, err := encAfter.Decrypt(reEncrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)
}

func TestDecryptAny(t *testing.T) {
	key1 := generateKey(t)
	key2 := generateKey(t)

	// Encrypt with key1 as active
	enc1, err := NewEncryptor(key1, "")
	require.NoError(t, err)
	encrypted, err := enc1.Encrypt([]byte("secret"))
	require.NoError(t, err)

	// Now key2 is active, key1 is old
	enc2, err := NewEncryptor(key2, key1)
	require.NoError(t, err)

	// DecryptAny should handle data encrypted with the now-old key
	decrypted, err := enc2.DecryptAny(encrypted)
	require.NoError(t, err)
	assert.Equal(t, []byte("secret"), decrypted)
}

func TestDecryptWrongKey(t *testing.T) {
	enc1, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	enc2, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	encrypted, err := enc1.Encrypt([]byte("secret"))
	require.NoError(t, err)

	_, err = enc2.Decrypt(encrypted)
	assert.Error(t, err)
}

func TestDecryptCorruptedData(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	t.Run("invalid json", func(t *testing.T) {
		_, err := enc.Decrypt([]byte("not json"))
		assert.Error(t, err)
	})

	t.Run("corrupted ciphertext", func(t *testing.T) {
		encrypted, err := enc.Encrypt([]byte("secret"))
		require.NoError(t, err)

		var env Envelope
		require.NoError(t, json.Unmarshal(encrypted, &env))
		env.Ciphertext = base64.StdEncoding.EncodeToString([]byte("corrupted"))
		corrupted, _ := json.Marshal(env)

		_, err = enc.Decrypt(corrupted)
		assert.Error(t, err)
	})

	t.Run("no old key for old slot", func(t *testing.T) {
		env := Envelope{KeySlot: "old", Nonce: "AAAA", Ciphertext: "BBBB"}
		data, _ := json.Marshal(env)

		_, err := enc.Decrypt(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no old key configured")
	})
}

func TestIsEnvelope(t *testing.T) {
	assert.True(t, IsEnvelope([]byte(`{"key":"active","nonce":"x","ciphertext":"y"}`)))
	assert.True(t, IsEnvelope([]byte(`{"key":"old","nonce":"x","ciphertext":"y"}`)))
	assert.False(t, IsEnvelope([]byte(`{"type":"bearer_token","bearer_token":{"token":"x"}}`)))
	assert.False(t, IsEnvelope([]byte(`not json`)))
	assert.False(t, IsEnvelope([]byte(`{"key":"unknown"}`)))
}

func TestNeedsRotation(t *testing.T) {
	assert.False(t, NeedsRotation([]byte(`{"key":"active"}`)))
	assert.True(t, NeedsRotation([]byte(`{"key":"old"}`)))
	assert.False(t, NeedsRotation([]byte(`not json`)))
}

func TestDecryptAnyNoFallback(t *testing.T) {
	key1 := generateKey(t)
	key2 := generateKey(t)

	enc1, err := NewEncryptor(key1, "")
	require.NoError(t, err)

	encrypted, err := enc1.Encrypt([]byte("secret"))
	require.NoError(t, err)

	// Encryptor with completely different key and no old key.
	enc2, err := NewEncryptor(key2, "")
	require.NoError(t, err)

	_, err = enc2.DecryptAny(encrypted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no fallback key available")
}

func TestDecryptAnyInvalidJSON(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	_, err = enc.DecryptAny([]byte("not json"))
	assert.Error(t, err)
}

func TestDecryptUnknownSlot(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	env := Envelope{KeySlot: "unknown", Nonce: "AAAA", Ciphertext: "BBBB"}
	data, _ := json.Marshal(env)

	_, err = enc.Decrypt(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key slot")
}

func TestDecryptInvalidNonce(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	env := Envelope{KeySlot: "active", Nonce: "not-valid-base64!!!", Ciphertext: "AAAA"}
	data, _ := json.Marshal(env)

	_, err = enc.Decrypt(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding nonce")
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), "")
	require.NoError(t, err)

	env := Envelope{KeySlot: "active", Nonce: base64.StdEncoding.EncodeToString(make([]byte, 12)), Ciphertext: "not-valid-base64!!!"}
	data, _ := json.Marshal(env)

	_, err = enc.Decrypt(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding ciphertext")
}

func TestEnvelopeAlwaysUsesActiveSlot(t *testing.T) {
	enc, err := NewEncryptor(generateKey(t), generateKey(t))
	require.NoError(t, err)

	encrypted, err := enc.Encrypt([]byte("test"))
	require.NoError(t, err)

	var env Envelope
	require.NoError(t, json.Unmarshal(encrypted, &env))
	assert.Equal(t, "active", env.KeySlot)
}
