package domain

import (
	"testing"
	"time"
)

func ptr(t time.Time) *time.Time { return &t }

func TestPasswordResetTokenState(t *testing.T) {
	now := time.Now()
	future := now.Add(30 * time.Minute)
	past := now.Add(-time.Minute)

	cases := []struct {
		name string
		tok  PasswordResetToken
		want PasswordResetTokenState
	}{
		{"fresh", PasswordResetToken{ExpiresAt: future}, PasswordResetTokenValid},
		{"expired", PasswordResetToken{ExpiresAt: past}, PasswordResetTokenExpired},
		{"consumed", PasswordResetToken{ExpiresAt: future, ConsumedAt: ptr(now)}, PasswordResetTokenUsed},
		{"invalidated", PasswordResetToken{ExpiresAt: future, InvalidatedAt: ptr(now)}, PasswordResetTokenInvalidated},
		// Consumed wins over expired: a used link should read "used", which is
		// the more accurate thing to tell a confused user.
		{"consumed and expired", PasswordResetToken{ExpiresAt: past, ConsumedAt: ptr(now)}, PasswordResetTokenUsed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tok.State(now); got != c.want {
				t.Fatalf("State() = %q, want %q", got, c.want)
			}
			if usable := c.tok.Usable(now); usable != (c.want == PasswordResetTokenValid) {
				t.Fatalf("Usable() = %v for state %q", usable, c.want)
			}
		})
	}
}

func TestPasswordResetTokenExpiryIsExclusive(t *testing.T) {
	now := time.Now()
	// A token expiring exactly now is spent, not valid.
	tok := PasswordResetToken{ExpiresAt: now}
	if tok.Usable(now) {
		t.Fatal("token expiring at the current instant must not be usable")
	}
}
