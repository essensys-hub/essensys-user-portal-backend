// Package pwreset holds the password reset token primitives shared by the
// self-service identity flow and the admin assist action. It deliberately owns
// no storage and no HTTP concern so both callers can depend on it without
// importing each other.
package pwreset

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// TTL is short on purpose: the link is the only thing standing between an
// intercepted mailbox and an account takeover.
const TTL = 60 * time.Minute

// MinPasswordLength matches the floor already applied at registration.
const MinPasswordLength = 8

const defaultPortalURL = "https://mon.essensys.fr"

// tokenBytes yields 256 bits of entropy, encoded to 43 URL-safe characters.
const tokenBytes = 32

// Generate returns the clear-text token to put in the email and the digest to
// store. The clear text is never persisted or logged.
func Generate() (plain string, digest string, err error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate reset token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, Hash(plain), nil
}

// Burn does the same work as Generate and throws the result away. It keeps the
// unknown-account branch of the forgot endpoint indistinguishable from the
// known-account branch by timing.
func Burn() {
	buf := make([]byte, tokenBytes)
	_, _ = rand.Read(buf)
	_ = Hash(base64.RawURLEncoding.EncodeToString(buf))
}

func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// PortalBaseURL mirrors the resolution already used for transactional email
// variables, so a reset link and a welcome link never disagree on the host.
func PortalBaseURL() string {
	base := strings.TrimSpace(os.Getenv("FRONTEND_URL"))
	if base == "" {
		return defaultPortalURL
	}
	return strings.TrimRight(base, "/")
}

func BuildResetURL(base, token string) string {
	if base == "" {
		base = defaultPortalURL
	}
	return fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(base, "/"), url.QueryEscape(token))
}

// MaskEmail keeps enough of the address for a user to recognise their own
// account without disclosing it to whoever holds the link.
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local, domain := email[:at], email[at:]
	if len(local) <= 1 {
		return "*" + domain
	}
	return local[:1] + strings.Repeat("*", 3) + domain
}

// ExpiresInMinutes is what the {{expires_in}} template variable renders to.
func ExpiresInMinutes(expiresAt time.Time, now time.Time) int {
	remaining := int(expiresAt.Sub(now).Minutes())
	if remaining < 0 {
		return 0
	}
	return remaining
}
