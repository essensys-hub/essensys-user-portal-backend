package domain

import "time"

// PasswordResetTokenState classifies why a token cannot be used, so callers can
// tell the user whether to expect a working link or ask for a new one.
type PasswordResetTokenState string

const (
	PasswordResetTokenValid       PasswordResetTokenState = "valid"
	PasswordResetTokenExpired     PasswordResetTokenState = "expired"
	PasswordResetTokenUsed        PasswordResetTokenState = "used"
	PasswordResetTokenInvalidated PasswordResetTokenState = "invalidated"
)

// PasswordResetToken never carries the clear-text token: only its SHA-256 hex
// digest is persisted and loaded.
type PasswordResetToken struct {
	ID            int        `db:"id"`
	UserID        int        `db:"user_id"`
	TokenHash     string     `db:"token_hash"`
	ExpiresAt     time.Time  `db:"expires_at"`
	ConsumedAt    *time.Time `db:"consumed_at"`
	InvalidatedAt *time.Time `db:"invalidated_at"`
	RequestIP     *string    `db:"request_ip"`
	ConsumedIP    *string    `db:"consumed_ip"`
	CreatedAt     time.Time  `db:"created_at"`
}

func (t *PasswordResetToken) State(now time.Time) PasswordResetTokenState {
	switch {
	case t == nil:
		return PasswordResetTokenInvalidated
	case t.ConsumedAt != nil:
		return PasswordResetTokenUsed
	case t.InvalidatedAt != nil:
		return PasswordResetTokenInvalidated
	case !t.ExpiresAt.After(now):
		return PasswordResetTokenExpired
	default:
		return PasswordResetTokenValid
	}
}

func (t *PasswordResetToken) Usable(now time.Time) bool {
	return t.State(now) == PasswordResetTokenValid
}
