// Package remember holds the remembered-session token primitives shared by the
// identity handlers and the stores. It owns no storage and no HTTP concern.
package remember

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// Lifetime is fixed from the moment the password was typed. Using or refreshing
// a session never extends it.
const Lifetime = 30 * 24 * time.Hour

// MaxSessionsPerUser caps active sessions per account: one more revokes the
// oldest.
const MaxSessionsPerUser = 10

// tokenBytes yields 256 bits of entropy, encoded to 43 URL-safe characters.
const tokenBytes = 32

// Generate returns the clear-text token to hand to the app and the digest to
// store. The clear text is never persisted or logged.
func Generate() (plain string, digest string, err error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate remember token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, Hash(plain), nil
}

func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
