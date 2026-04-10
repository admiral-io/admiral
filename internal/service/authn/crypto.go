package authn

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

type cryptographer struct {
	gcm cipher.AEAD
}

func newCryptographer(passphrase string) (*cryptographer, error) {
	if len(passphrase) == 0 {
		return nil, fmt.Errorf("passphrase was empty")
	}

	key := sha256.Sum256([]byte(passphrase))

	cb, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(cb)
	if err != nil {
		return nil, err
	}

	return &cryptographer{gcm: gcm}, nil
}

func (c *cryptographer) Decrypt(b []byte) ([]byte, error) {
	if len(b) < c.gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce+cipher, bytes for decryption are smaller than algorithm's nonce size")
	}

	nonce, ciphertext := b[:c.gcm.NonceSize()], b[c.gcm.NonceSize():]

	out, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *cryptographer) Encrypt(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("input bytes were empty, could not encrypt")
	}

	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("error generating nonce: %w", err)
	}

	out := make([]byte, c.gcm.NonceSize(), c.gcm.NonceSize()+c.gcm.Overhead()+len(b))
	copy(out, nonce)

	return c.gcm.Seal(out, nonce, b, nil), nil
}
