package authn

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCryptographer(t *testing.T) {
	testCases := []struct {
		name        string
		passphrase  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid passphrase",
			passphrase:  "test-passphrase",
			expectError: false,
		},
		{
			name:        "long passphrase",
			passphrase:  "this-is-a-very-long-passphrase-that-should-work-perfectly-fine",
			expectError: false,
		},
		{
			name:        "short passphrase",
			passphrase:  "a",
			expectError: false,
		},
		{
			name:        "unicode passphrase",
			passphrase:  "测试密码🔐",
			expectError: false,
		},
		{
			name:        "passphrase with spaces",
			passphrase:  "test passphrase with spaces",
			expectError: false,
		},
		{
			name:        "passphrase with special characters",
			passphrase:  "!@#$%^&*()_+-=[]{}|;:,.<>?",
			expectError: false,
		},
		{
			name:        "empty passphrase",
			passphrase:  "",
			expectError: true,
			errorMsg:    "passphrase was empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cryptographer, err := newCryptographer(tc.passphrase)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, cryptographer)
				if tc.errorMsg != "" {
					assert.Equal(t, tc.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cryptographer)
				assert.NotNil(t, cryptographer.gcm)

				// Verify GCM properties
				assert.Equal(t, 12, cryptographer.gcm.NonceSize()) // AES-GCM standard nonce size
				assert.Equal(t, 16, cryptographer.gcm.Overhead())  // AES-GCM standard overhead
			}
		})
	}
}

func TestNewCryptographer_SamePassphraseDifferentInstances(t *testing.T) {
	passphrase := "test-passphrase"

	crypto1, err1 := newCryptographer(passphrase)
	crypto2, err2 := newCryptographer(passphrase)

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotNil(t, crypto1)
	require.NotNil(t, crypto2)

	// Different instances but same underlying cipher
	assert.NotSame(t, crypto1, crypto2)
	assert.NotSame(t, crypto1.gcm, crypto2.gcm)

	// But they should produce the same key internally (verified by round-trip compatibility)
	plaintext := []byte("test message")

	encrypted1, err := crypto1.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted2, err := crypto2.Decrypt(encrypted1)
	require.NoError(t, err)

	assert.Equal(t, plaintext, decrypted2)
}

func TestCryptographer_Encrypt(t *testing.T) {
	cryptographer, err := newCryptographer("test-passphrase")
	require.NoError(t, err)
	require.NotNil(t, cryptographer)

	testCases := []struct {
		name        string
		input       []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "simple text",
			input:       []byte("hello world"),
			expectError: false,
		},
		{
			name:        "empty string (not empty bytes)",
			input:       []byte(""),
			expectError: true,
			errorMsg:    "input bytes were empty, could not encrypt",
		},
		{
			name:        "nil input",
			input:       nil,
			expectError: true,
			errorMsg:    "input bytes were empty, could not encrypt",
		},
		{
			name:        "single byte",
			input:       []byte("a"),
			expectError: false,
		},
		{
			name:        "large text",
			input:       []byte(strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 1000)),
			expectError: false,
		},
		{
			name:        "binary data",
			input:       []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
			expectError: false,
		},
		{
			name:        "unicode text",
			input:       []byte("Hello 世界 🌍 🔐"),
			expectError: false,
		},
		{
			name:        "json data",
			input:       []byte(`{"name":"test","value":123,"nested":{"key":"value"}}`),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := cryptographer.Encrypt(tc.input)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, encrypted)
				if tc.errorMsg != "" {
					assert.Equal(t, tc.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, encrypted)

				// Verify encrypted data properties
				expectedMinSize := cryptographer.gcm.NonceSize() + cryptographer.gcm.Overhead() + len(tc.input)
				assert.GreaterOrEqual(t, len(encrypted), expectedMinSize)

				// Verify it's different from input (unless empty)
				if len(tc.input) > 0 {
					assert.NotEqual(t, tc.input, encrypted)
				}

				// Verify nonce is at the beginning and looks random
				nonce := encrypted[:cryptographer.gcm.NonceSize()]
				assert.Equal(t, cryptographer.gcm.NonceSize(), len(nonce))
				// Nonce should not be all zeros (extremely unlikely with crypto/rand)
				assert.NotEqual(t, make([]byte, cryptographer.gcm.NonceSize()), nonce)
			}
		})
	}
}

func TestCryptographer_Decrypt(t *testing.T) {
	cryptographer, err := newCryptographer("test-passphrase")
	require.NoError(t, err)
	require.NotNil(t, cryptographer)

	// Create some valid encrypted data for corruption tests
	plaintext := []byte("test message for decryption")
	validEncrypted, err := cryptographer.Encrypt(plaintext)
	require.NoError(t, err)

	testCases := []struct {
		name        string
		setupInput  func() []byte
		expected    []byte
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid encrypted data",
			setupInput: func() []byte {
				plaintext := []byte("test message for decryption")
				encrypted, err := cryptographer.Encrypt(plaintext)
				require.NoError(t, err)
				return encrypted
			},
			expected:    []byte("test message for decryption"),
			expectError: false,
		},
		{
			name:        "nil input",
			setupInput:  func() []byte { return nil },
			expected:    nil,
			expectError: true,
			errorMsg:    "invalid nonce+cipher, bytes for decryption are smaller than algorithm's nonce size",
		},
		{
			name:        "empty input",
			setupInput:  func() []byte { return []byte{} },
			expected:    nil,
			expectError: true,
			errorMsg:    "invalid nonce+cipher, bytes for decryption are smaller than algorithm's nonce size",
		},
		{
			name:        "too short input (smaller than nonce)",
			setupInput:  func() []byte { return []byte{0x01, 0x02, 0x03} }, // Less than 12 bytes
			expected:    nil,
			expectError: true,
			errorMsg:    "invalid nonce+cipher, bytes for decryption are smaller than algorithm's nonce size",
		},
		{
			name:        "exactly nonce size (no ciphertext)",
			setupInput:  func() []byte { return make([]byte, 12) }, // Exactly nonce size, no ciphertext
			expected:    nil,
			expectError: true,
		},
		{
			name: "corrupted nonce",
			setupInput: func() []byte {
				corrupted := make([]byte, len(validEncrypted))
				copy(corrupted, validEncrypted)
				corrupted[0] ^= 0x01 // Flip one bit in nonce
				return corrupted
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "corrupted ciphertext",
			setupInput: func() []byte {
				corrupted := make([]byte, len(validEncrypted))
				copy(corrupted, validEncrypted)
				if len(corrupted) > 12 {
					corrupted[13] ^= 0x01 // Flip one bit in ciphertext
				}
				return corrupted
			},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "completely random data",
			setupInput:  func() []byte { return make([]byte, 50) }, // Zeros, should fail authentication
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.setupInput()
			decrypted, err := cryptographer.Decrypt(input)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Equal(t, tc.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, decrypted)
			}
		})
	}
}

func TestCryptographer_EncryptDecryptRoundTrip(t *testing.T) {
	cryptographer, err := newCryptographer("test-passphrase")
	require.NoError(t, err)

	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{
			name:      "simple text",
			plaintext: []byte("hello world"),
		},
		{
			name:      "single character",
			plaintext: []byte("a"),
		},
		{
			name:      "numbers and symbols",
			plaintext: []byte("1234567890!@#$%^&*()"),
		},
		{
			name:      "unicode text",
			plaintext: []byte("Hello 世界 🌍"),
		},
		{
			name:      "large text",
			plaintext: []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)),
		},
		{
			name:      "binary data",
			plaintext: []byte{0x00, 0x01, 0x7F, 0x80, 0xFF, 0xFE, 0xFD, 0xFC},
		},
		{
			name:      "json structure",
			plaintext: []byte(`{"user":{"id":123,"name":"John Doe","active":true},"timestamp":"2023-01-01T00:00:00Z"}`),
		},
		{
			name:      "very large data",
			plaintext: make([]byte, 10000), // Will be initialized to zeros
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fill large data with random pattern for more realistic test
			if len(tc.plaintext) == 10000 {
				for i := range tc.plaintext {
					tc.plaintext[i] = byte(i % 256)
				}
			}

			// Encrypt
			encrypted, err := cryptographer.Encrypt(tc.plaintext)
			require.NoError(t, err)
			require.NotNil(t, encrypted)

			// Verify encrypted is different from plaintext
			assert.NotEqual(t, tc.plaintext, encrypted)

			// Verify encrypted has expected size
			expectedMinSize := cryptographer.gcm.NonceSize() + cryptographer.gcm.Overhead() + len(tc.plaintext)
			assert.GreaterOrEqual(t, len(encrypted), expectedMinSize)

			// Decrypt
			decrypted, err := cryptographer.Decrypt(encrypted)
			require.NoError(t, err)

			// Verify round trip
			assert.Equal(t, tc.plaintext, decrypted)
		})
	}
}

func TestCryptographer_MultipleEncryptions(t *testing.T) {
	cryptographer, err := newCryptographer("test-passphrase")
	require.NoError(t, err)

	plaintext := []byte("same message encrypted multiple times")

	// Encrypt the same message multiple times
	var encrypted [][]byte
	for i := 0; i < 10; i++ {
		enc, err := cryptographer.Encrypt(plaintext)
		require.NoError(t, err)
		encrypted = append(encrypted, enc)
	}

	// Each encryption should be different (due to random nonce)
	for i := 0; i < len(encrypted); i++ {
		for j := i + 1; j < len(encrypted); j++ {
			assert.NotEqual(t, encrypted[i], encrypted[j], "encryptions %d and %d should be different", i, j)
		}
	}

	// But all should decrypt to the same plaintext
	for i, enc := range encrypted {
		decrypted, err := cryptographer.Decrypt(enc)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted, "decryption %d should match original", i)
	}
}

func TestCryptographer_DifferentPassphrases(t *testing.T) {
	crypto1, err := newCryptographer("passphrase1")
	require.NoError(t, err)

	crypto2, err := newCryptographer("passphrase2")
	require.NoError(t, err)

	plaintext := []byte("message encrypted with different keys")

	// Encrypt with first cryptographer
	encrypted, err := crypto1.Encrypt(plaintext)
	require.NoError(t, err)

	// Try to decrypt with second cryptographer (should fail)
	_, err = crypto2.Decrypt(encrypted)
	assert.Error(t, err, "decryption with different key should fail")

	// Verify correct cryptographer can still decrypt
	decrypted, err := crypto1.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestCryptographer_ConcurrentAccess(t *testing.T) {
	cryptographer, err := newCryptographer("test-passphrase")
	require.NoError(t, err)

	// Test concurrent encryption/decryption
	const numGoroutines = 10
	const numIterations = 10

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*numIterations*2) // *2 for encrypt+decrypt

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for i := 0; i < numIterations; i++ {
				plaintext := []byte("concurrent test message from goroutine " + string(rune('0'+goroutineID)) + " iteration " + string(rune('0'+i)))

				// Encrypt
				encrypted, err := cryptographer.Encrypt(plaintext)
				if err != nil {
					errors <- err
					return
				}

				// Decrypt
				decrypted, err := cryptographer.Decrypt(encrypted)
				if err != nil {
					errors <- err
					return
				}

				if !bytes.Equal(plaintext, decrypted) {
					errors <- assert.AnError
					return
				}
			}
		}(g)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check for any errors
	close(errors)
	for err := range errors {
		t.Errorf("Concurrent test error: %v", err)
	}
}

func TestCryptographer_EdgeCases(t *testing.T) {
	cryptographer, err := newCryptographer("test-passphrase")
	require.NoError(t, err)

	t.Run("encrypt single byte values", func(t *testing.T) {
		for i := 0; i < 256; i++ {
			plaintext := []byte{byte(i)}

			encrypted, err := cryptographer.Encrypt(plaintext)
			require.NoError(t, err)

			decrypted, err := cryptographer.Decrypt(encrypted)
			require.NoError(t, err)

			assert.Equal(t, plaintext, decrypted)
		}
	})

	t.Run("decrypt with wrong cryptographer instance", func(t *testing.T) {
		// Create two different cryptographers with same passphrase
		crypto1, err := newCryptographer("same-passphrase")
		require.NoError(t, err)

		crypto2, err := newCryptographer("same-passphrase")
		require.NoError(t, err)

		plaintext := []byte("test message")
		encrypted, err := crypto1.Encrypt(plaintext)
		require.NoError(t, err)

		// Should work with different instance but same passphrase
		decrypted, err := crypto2.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("very long passphrase", func(t *testing.T) {
		longPassphrase := strings.Repeat("very-long-passphrase-", 100)
		longCrypto, err := newCryptographer(longPassphrase)
		require.NoError(t, err)

		plaintext := []byte("message with very long passphrase")
		encrypted, err := longCrypto.Encrypt(plaintext)
		require.NoError(t, err)

		decrypted, err := longCrypto.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})
}

func TestCryptographer_SecurityProperties(t *testing.T) {
	cryptographer, err := newCryptographer("test-passphrase")
	require.NoError(t, err)

	t.Run("nonce uniqueness", func(t *testing.T) {
		plaintext := []byte("same message")
		nonces := make(map[string]bool)

		// Generate multiple encryptions and collect nonces
		for i := 0; i < 100; i++ {
			encrypted, err := cryptographer.Encrypt(plaintext)
			require.NoError(t, err)

			nonce := encrypted[:cryptographer.gcm.NonceSize()]
			nonceStr := string(nonce)

			// Each nonce should be unique
			assert.False(t, nonces[nonceStr], "nonce collision detected at iteration %d", i)
			nonces[nonceStr] = true
		}
	})

	t.Run("ciphertext authenticity", func(t *testing.T) {
		plaintext := []byte("authenticated message")
		encrypted, err := cryptographer.Encrypt(plaintext)
		require.NoError(t, err)

		// Modify one bit in the ciphertext (after nonce)
		nonceSize := cryptographer.gcm.NonceSize()
		if len(encrypted) > nonceSize {
			corrupted := make([]byte, len(encrypted))
			copy(corrupted, encrypted)
			corrupted[nonceSize] ^= 0x01 // Flip one bit

			// Decryption should fail due to authentication failure
			_, err := cryptographer.Decrypt(corrupted)
			assert.Error(t, err, "decryption of modified ciphertext should fail")
		}
	})

	t.Run("nonce modification detection", func(t *testing.T) {
		plaintext := []byte("nonce protection test")
		encrypted, err := cryptographer.Encrypt(plaintext)
		require.NoError(t, err)

		// Modify nonce
		corrupted := make([]byte, len(encrypted))
		copy(corrupted, encrypted)
		corrupted[0] ^= 0x01 // Flip one bit in nonce

		// Decryption should fail
		_, err = cryptographer.Decrypt(corrupted)
		assert.Error(t, err, "decryption with modified nonce should fail")
	})
}

func TestCryptographer_StructProperties(t *testing.T) {
	t.Run("cryptographer struct fields", func(t *testing.T) {
		crypto, err := newCryptographer("test-passphrase")
		require.NoError(t, err)

		// Verify struct has expected field
		assert.NotNil(t, crypto.gcm)

		// Verify GCM interface methods are available
		assert.Equal(t, 12, crypto.gcm.NonceSize())
		assert.Equal(t, 16, crypto.gcm.Overhead())
	})

	t.Run("nil cryptographer methods", func(t *testing.T) {
		var crypto *cryptographer = nil

		// These should panic or fail gracefully
		assert.Panics(t, func() {
			_, _ = crypto.Encrypt([]byte("test"))
		})

		assert.Panics(t, func() {
			_, _ = crypto.Decrypt([]byte("test"))
		})
	})
}

func BenchmarkCryptographer_Encrypt(b *testing.B) {
	cryptographer, err := newCryptographer("benchmark-passphrase")
	require.NoError(b, err)

	testData := []byte(strings.Repeat("benchmark test data ", 50)) // ~1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cryptographer.Encrypt(testData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCryptographer_Decrypt(b *testing.B) {
	cryptographer, err := newCryptographer("benchmark-passphrase")
	require.NoError(b, err)

	testData := []byte(strings.Repeat("benchmark test data ", 50)) // ~1KB
	encrypted, err := cryptographer.Encrypt(testData)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cryptographer.Decrypt(encrypted)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewCryptographer(b *testing.B) {
	passphrase := "benchmark-passphrase"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := newCryptographer(passphrase)
		if err != nil {
			b.Fatal(err)
		}
	}
}
