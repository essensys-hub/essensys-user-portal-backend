package domain

import "time"

// RememberSession is one remembered login on one device. The clear token is
// never stored: TokenHash is its SHA-256 digest.
type RememberSession struct {
	ID         int        `db:"id"`
	UserID     int        `db:"user_id"`
	TokenHash  string     `db:"token_hash"`
	CreatedAt  time.Time  `db:"created_at"`
	ExpiresAt  time.Time  `db:"expires_at"`
	LastUsedAt *time.Time `db:"last_used_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
	UserAgent  *string    `db:"user_agent"`
}

// Usable is the single place that decides whether a session may be exchanged.
// A session expires at ExpiresAt exactly, and a revoked one is never usable.
func (s *RememberSession) Usable(now time.Time) bool {
	return s != nil && s.RevokedAt == nil && now.Before(s.ExpiresAt)
}
