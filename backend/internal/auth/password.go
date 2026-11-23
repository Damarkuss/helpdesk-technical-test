// Package auth contains password hashing and JWT handling using only the
// Go standard library.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	hashScheme = "pbkdf2-sha256"
	saltLen    = 16
	keyLen     = 32

	// DefaultIterations follows the OWASP recommendation for PBKDF2-HMAC-SHA256.
	DefaultIterations = 600_000
)

// Hasher hashes and verifies passwords with salted PBKDF2-HMAC-SHA256.
// The stored format is "pbkdf2-sha256$<iterations>$<salt>$<key>" so the
// iteration count can be raised later without invalidating old hashes.
type Hasher struct {
	iterations int
}

// NewHasher returns a Hasher; iterations <= 0 selects DefaultIterations.
func NewHasher(iterations int) *Hasher {
	if iterations <= 0 {
		iterations = DefaultIterations
	}
	return &Hasher{iterations: iterations}
}

// Hash returns the encoded hash of password using a fresh random salt.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, h.iterations, keyLen)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	enc := base64.RawStdEncoding
	return strings.Join([]string{hashScheme, strconv.Itoa(h.iterations), enc.EncodeToString(salt), enc.EncodeToString(key)}, "$"), nil
}

// Verify checks password against an encoded hash in constant time.
// Malformed hashes simply fail verification.
func (h *Hasher) Verify(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != hashScheme {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := enc.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
