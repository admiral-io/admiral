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
	tokenPrefixLen  = 12
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

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

func HashOpaqueToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

func IsOpaqueToken(raw string) bool {
	return strings.HasPrefix(raw, PrefixPAT) || strings.HasPrefix(raw, PrefixSAT) || strings.HasPrefix(raw, PrefixSession)
}

func ValidateChecksum(raw string) bool {
	if len(raw) <= checksumLen {
		return false
	}

	body := raw[:len(raw)-checksumLen]
	expected := encodeBase62CRC32(body)
	return raw[len(raw)-checksumLen:] == expected
}

func tokenPrefixFromPlaintext(plaintext string) string {
	if len(plaintext) <= tokenPrefixLen {
		return plaintext
	}
	return plaintext[:tokenPrefixLen]
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

func encodeBase62CRC32(s string) string {
	n := crc32.ChecksumIEEE([]byte(s))
	buf := make([]byte, checksumLen)
	for i := checksumLen - 1; i >= 0; i-- {
		buf[i] = base62[n%62]
		n /= 62
	}
	return string(buf)
}
