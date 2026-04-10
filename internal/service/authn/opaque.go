package authn

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"strings"
)

const (
	PrefixPAT       = "admp_"
	PrefixSAT       = "adms_"
	PrefixSession   = "adme_"
	opaqueRandBytes = 32
	checksumLen     = 6
)

// base62 alphabet for CRC32 checksum encoding.
const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateOpaqueToken creates a prefixed opaque token with a CRC32 checksum
// and returns the plaintext and its SHA-256 hash for DB storage.
//
// Format: <prefix><base64url random><6-char base62 CRC32>
// Example: admp_abc123...xyz_A1b2C3.
func GenerateOpaqueToken(kind TokenKind) (plaintext string, hash []byte, err error) {
	prefix, err := prefixForKind(kind)
	if err != nil {
		return "", nil, err
	}

	b := make([]byte, opaqueRandBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	body := prefix + base64.RawURLEncoding.EncodeToString(b)
	checksum := encodeBase62CRC32(body)
	plaintext = body + checksum

	h := sha256.Sum256([]byte(plaintext))
	return plaintext, h[:], nil
}

// HashOpaqueToken computes the SHA-256 hash of a raw opaque token string.
func HashOpaqueToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

// IsOpaqueToken returns true if the token has an Admiral opaque prefix.
func IsOpaqueToken(raw string) bool {
	return strings.HasPrefix(raw, PrefixPAT) || strings.HasPrefix(raw, PrefixSAT) || strings.HasPrefix(raw, PrefixSession)
}

// ValidateChecksum verifies the CRC32 checksum suffix of an opaque token.
// Returns true if the checksum is valid. Does not require a DB lookup.
func ValidateChecksum(raw string) bool {
	if len(raw) <= checksumLen {
		return false
	}

	body := raw[:len(raw)-checksumLen]
	expected := encodeBase62CRC32(body)
	return raw[len(raw)-checksumLen:] == expected
}

func prefixForKind(kind TokenKind) (string, error) {
	switch kind {
	case TokenKindPAT:
		return PrefixPAT, nil
	case TokenKindSAT:
		return PrefixSAT, nil
	case TokenKindSession:
		return PrefixSession, nil
	default:
		return "", fmt.Errorf("opaque tokens not supported for kind %q", kind)
	}
}

// encodeBase62CRC32 computes CRC32 of s and encodes it as a fixed-length
// base62 string (zero-padded to checksumLen characters).
func encodeBase62CRC32(s string) string {
	n := crc32.ChecksumIEEE([]byte(s))
	buf := make([]byte, checksumLen)
	for i := checksumLen - 1; i >= 0; i-- {
		buf[i] = base62[n%62]
		n /= 62
	}
	return string(buf)
}
