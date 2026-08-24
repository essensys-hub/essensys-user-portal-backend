package data

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/pwreset"
	"github.com/jmoiron/sqlx"
)

// ErrTokenNotConsumable is returned when the conditional consume matched no
// row: the token was already used, invalidated, or has expired. Callers must
// treat it as an invalid token and never disclose which of the three it was.
var ErrTokenNotConsumable = errors.New("password reset token is not consumable")

// retention is how long spent tokens stay queryable for diagnostics.
const retention = 30 * 24 * time.Hour

type PasswordResetStore struct {
	db *sqlx.DB
}

func NewPasswordResetStore(db *sqlx.DB) *PasswordResetStore {
	return &PasswordResetStore{db: db}
}

func (s *PasswordResetStore) EnsureTableExists() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS password_reset_tokens (
		id             SERIAL PRIMARY KEY,
		user_id        INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash     CHAR(64) NOT NULL UNIQUE,
		expires_at     TIMESTAMPTZ NOT NULL,
		consumed_at    TIMESTAMPTZ,
		invalidated_at TIMESTAMPTZ,
		request_ip     VARCHAR(64),
		consumed_ip    VARCHAR(64),
		created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
	CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_created_at ON password_reset_tokens(created_at DESC);
	`)
	return err
}

func (s *PasswordResetStore) Create(userID int, tokenHash string, expiresAt time.Time, requestIP string) error {
	_, err := s.db.Exec(`
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, request_ip)
		VALUES ($1, $2, $3, $4)`, userID, tokenHash, expiresAt, nullIfEmpty(requestIP))
	return err
}

// Issue mints a token for an account and returns the clear text for the caller
// to put in an email. The clear text is never persisted or logged. Shared by the
// self-service flow and the admin assist action so both obey the same rules.
func (s *PasswordResetStore) Issue(userID int, requestIP string) (plain string, expiresAt time.Time, err error) {
	plain, digest, err := pwreset.Generate()
	if err != nil {
		return "", time.Time{}, err
	}
	// Retire outstanding links first: issuing a new one must make every earlier
	// one unusable, otherwise an old mail stays a valid way in.
	if err := s.InvalidateForUser(userID); err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().Add(pwreset.TTL)
	if err := s.Create(userID, digest, expiresAt, requestIP); err != nil {
		return "", time.Time{}, err
	}
	if err := s.PurgeOlderThan(retention); err != nil {
		log.Printf("[pwreset] purge: %v", err)
	}
	return plain, expiresAt, nil
}

func (s *PasswordResetStore) GetByHash(tokenHash string) (*domain.PasswordResetToken, error) {
	var tok domain.PasswordResetToken
	err := s.db.Get(&tok, `SELECT * FROM password_reset_tokens WHERE token_hash = $1`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tok, nil
}

// InvalidateForUser retires every outstanding token of an account. Called both
// when a fresh token is issued and when a reset completes, so a mailbox holding
// several old links cannot be replayed.
func (s *PasswordResetStore) InvalidateForUser(userID int) error {
	_, err := s.db.Exec(`
		UPDATE password_reset_tokens SET invalidated_at = NOW()
		WHERE user_id = $1 AND consumed_at IS NULL AND invalidated_at IS NULL`, userID)
	return err
}

// CountRecentForUser backs the per-account issue limit, which stops an attacker
// from flooding one victim's mailbox from many source addresses.
func (s *PasswordResetStore) CountRecentForUser(userID int, window time.Duration) (int, error) {
	var count int
	err := s.db.Get(&count, `
		SELECT count(*) FROM password_reset_tokens
		WHERE user_id = $1 AND created_at > $2`, userID, time.Now().Add(-window))
	return count, err
}

// ConsumeAndSetPassword is the single-use guarantee. The conditional UPDATE plus
// the affected-row check means that when two requests race on the same token,
// exactly one observes a match and the loser gets ErrTokenNotConsumable. Doing
// the consume and the password write in one transaction also rules out a
// consumed token that never changed the password.
func (s *PasswordResetStore) ConsumeAndSetPassword(tokenHash, newPasswordHash, ip string) (userID int, err error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	err = tx.Get(&userID, `
		UPDATE password_reset_tokens
		SET consumed_at = NOW(), consumed_ip = $2
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND invalidated_at IS NULL
		  AND expires_at > NOW()
		RETURNING user_id`, tokenHash, nullIfEmpty(ip))
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrTokenNotConsumable
		return 0, err
	}
	if err != nil {
		return 0, err
	}

	if _, err = tx.Exec(`UPDATE users SET password_hash = $1 WHERE id = $2`, newPasswordHash, userID); err != nil {
		return 0, err
	}

	if _, err = tx.Exec(`
		UPDATE password_reset_tokens SET invalidated_at = NOW()
		WHERE user_id = $1 AND consumed_at IS NULL AND invalidated_at IS NULL`, userID); err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

// PurgeOlderThan drops rows with no diagnostic value left. Called lazily on
// issue rather than from a scheduled job: at a few resets per month a cron
// entry would be more moving parts than the problem warrants.
func (s *PasswordResetStore) PurgeOlderThan(age time.Duration) error {
	_, err := s.db.Exec(`DELETE FROM password_reset_tokens WHERE created_at < $1`, time.Now().Add(-age))
	return err
}
