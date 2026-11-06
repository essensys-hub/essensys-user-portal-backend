package data

import (
	"database/sql"
	"errors"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/remember"
	"github.com/jmoiron/sqlx"
)

// revokeAllRememberSQL ends every active remembered session of one account. It
// runs inside the same transaction as any credential write (see UserStore and
// PasswordResetStore), so a password can never change while an old session
// stays usable.
const revokeAllRememberSQL = `
	UPDATE remember_sessions SET revoked_at = NOW()
	WHERE user_id = $1 AND revoked_at IS NULL`

type RememberSessionStore struct {
	db *sqlx.DB
}

func NewRememberSessionStore(db *sqlx.DB) *RememberSessionStore {
	return &RememberSessionStore{db: db}
}

// Create stores the digest of a new session and, in the same transaction,
// revokes the oldest active ones beyond remember.MaxSessionsPerUser. now comes
// from the caller so the cap can be tested against a controlled clock.
func (s *RememberSessionStore) Create(userID int, tokenHash string, expiresAt time.Time, userAgent string, now time.Time) (err error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`
		INSERT INTO remember_sessions (user_id, token_hash, expires_at, user_agent)
		VALUES ($1, $2, $3, $4)`, userID, tokenHash, expiresAt, nullIfEmpty(userAgent)); err != nil {
		return err
	}
	if _, err = tx.Exec(`
		UPDATE remember_sessions SET revoked_at = $2
		WHERE id IN (
			SELECT id FROM remember_sessions
			WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
			ORDER BY created_at DESC, id DESC
			OFFSET $3
		)`, userID, now, remember.MaxSessionsPerUser); err != nil {
		return err
	}
	return tx.Commit()
}

// GetByHash returns nil, nil when no session carries this digest. Whether the
// session is still usable is decided by RememberSession.Usable.
func (s *RememberSessionStore) GetByHash(tokenHash string) (*domain.RememberSession, error) {
	var sess domain.RememberSession
	err := s.db.Get(&sess, `SELECT * FROM remember_sessions WHERE token_hash = $1`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// Touch only records the last use. expires_at is never written after Create:
// the 30 days are counted from the password entry, whatever the usage.
func (s *RememberSessionStore) Touch(id int, now time.Time) error {
	_, err := s.db.Exec(`UPDATE remember_sessions SET last_used_at = $2 WHERE id = $1`, id, now)
	return err
}

// RevokeByHash is idempotent: an unknown or already revoked digest is not an error.
func (s *RememberSessionStore) RevokeByHash(tokenHash string, now time.Time) error {
	_, err := s.db.Exec(`
		UPDATE remember_sessions SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash, now)
	return err
}
