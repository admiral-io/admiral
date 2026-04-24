package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Envelope is the JSON structure stored in place of plaintext auth_config.
// KeySlot identifies which key encrypted the data: "active" or "old".
type Envelope struct {
	KeySlot    string `json:"key"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// Encryptor provides AES-256-GCM encryption with two-key rotation support.
// The active key encrypts all new data. Both keys can decrypt.
type Encryptor struct {
	active    cipher.AEAD
	old       cipher.AEAD
	activeRaw []byte
	oldRaw    []byte
}

const (
	slotActive = "active"
	slotOld    = "old"
	keyLen     = 32 // AES-256
)

// NewEncryptor creates an Encryptor from a base64-encoded active key and an
// optional base64-encoded old key (empty string means no old key).
func NewEncryptor(activeKeyB64, oldKeyB64 string) (*Encryptor, error) {
	if activeKeyB64 == "" {
		return nil, fmt.Errorf("active encryption key is required")
	}

	activeKey, err := decodeKey(activeKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid active key: %w", err)
	}

	activeGCM, err := newGCM(activeKey)
	if err != nil {
		return nil, fmt.Errorf("active key: %w", err)
	}

	enc := &Encryptor{
		active:    activeGCM,
		activeRaw: activeKey,
	}

	if oldKeyB64 != "" {
		oldKey, err := decodeKey(oldKeyB64)
		if err != nil {
			return nil, fmt.Errorf("invalid old key: %w", err)
		}

		oldGCM, err := newGCM(oldKey)
		if err != nil {
			return nil, fmt.Errorf("old key: %w", err)
		}

		enc.old = oldGCM
		enc.oldRaw = oldKey
	}

	return enc, nil
}

// Encrypt encrypts plaintext bytes and returns a JSON-encoded Envelope.
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("plaintext is empty")
	}

	nonce := make([]byte, e.active.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := e.active.Seal(nil, nonce, plaintext, nil)

	env := Envelope{
		KeySlot:    slotActive,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}

	return json.Marshal(env)
}

// Decrypt decrypts a JSON-encoded Envelope back to plaintext bytes.
func (e *Encryptor) Decrypt(envelopeJSON []byte) ([]byte, error) {
	var env Envelope
	if err := json.Unmarshal(envelopeJSON, &env); err != nil {
		return nil, fmt.Errorf("invalid envelope: %w", err)
	}

	gcm, err := e.gcmForSlot(env.KeySlot)
	if err != nil {
		return nil, err
	}

	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decoding nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decoding ciphertext: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// IsEnvelope returns true if the JSON bytes look like an encrypted envelope
// (has a "key" field) rather than a plaintext AuthConfig (has a "type" field).
func IsEnvelope(data []byte) bool {
	var probe struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Key == slotActive || probe.Key == slotOld
}

// NeedsRotation returns true if the envelope was encrypted with the old key.
func NeedsRotation(data []byte) bool {
	var probe struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Key == slotOld
}

// DecryptAny decrypts an envelope by trying the slot-matched key first, then
// falling back to the other key. This handles the rotation window where data
// was encrypted with a key that has since changed slots (e.g. the old "active"
// key is now the "old" key after rotation).
func (e *Encryptor) DecryptAny(envelopeJSON []byte) ([]byte, error) {
	// Try the slot-matched key first.
	plaintext, err := e.Decrypt(envelopeJSON)
	if err == nil {
		return plaintext, nil
	}

	// Slot-matched key failed. Try the other key.
	var env Envelope
	if jsonErr := json.Unmarshal(envelopeJSON, &env); jsonErr != nil {
		return nil, fmt.Errorf("invalid envelope: %w", jsonErr)
	}

	nonce, err2 := base64.StdEncoding.DecodeString(env.Nonce)
	if err2 != nil {
		return nil, fmt.Errorf("decoding nonce: %w", err2)
	}
	ciphertext, err2 := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err2 != nil {
		return nil, fmt.Errorf("decoding ciphertext: %w", err2)
	}

	// Try the other key.
	var fallback cipher.AEAD
	switch env.KeySlot {
	case slotActive:
		fallback = e.old
	case slotOld:
		fallback = e.active
	}

	if fallback == nil {
		return nil, fmt.Errorf("decryption failed and no fallback key available: %w", err)
	}

	plaintext, err2 = fallback.Open(nil, nonce, ciphertext, nil)
	if err2 != nil {
		return nil, fmt.Errorf("decryption failed with both keys: %w", err)
	}

	return plaintext, nil
}

// HasOldKey returns true if an old key is configured.
func (e *Encryptor) HasOldKey() bool {
	return e.old != nil
}

func (e *Encryptor) gcmForSlot(slot string) (cipher.AEAD, error) {
	switch slot {
	case slotActive:
		return e.active, nil
	case slotOld:
		if e.old == nil {
			return nil, fmt.Errorf("data encrypted with old key but no old key configured")
		}
		return e.old, nil
	default:
		return nil, fmt.Errorf("unknown key slot: %q", slot)
	}
}

func decodeKey(b64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("key must be %d bytes, got %d", keyLen, len(key))
	}
	return key, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
