package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// Support report statuses, mirrored by the CHECK constraint of migration 015.
const (
	SupportStatusPending  = "pending"
	SupportStatusOpen     = "open"
	SupportStatusResolved = "resolved"
	SupportStatusClosed   = "closed"
)

// supportSyncLockKey is the pg advisory lock that keeps a single sync worker
// running when several backend instances share the database.
const supportSyncLockKey = 7426004

// SupportReport is one bug or incident filed from the support site.
type SupportReport struct {
	ID          int             `db:"id"`
	PublicRef   string          `db:"public_ref"`
	UserID      int             `db:"user_id"`
	Kind        string          `db:"kind"`
	Title       string          `db:"title"`
	Payload     json.RawMessage `db:"payload"`
	Status      string          `db:"status"`
	Repo        sql.NullString  `db:"repo"`
	IssueNumber sql.NullInt64   `db:"issue_number"`
	IssueURL    sql.NullString  `db:"issue_url"`
	Attempts    int             `db:"attempts"`
	LastError   sql.NullString  `db:"last_error"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
	SyncedAt    sql.NullTime    `db:"synced_at"`
}

const supportReportColumns = `id, public_ref, user_id, kind, title, payload, status, repo, issue_number,
	issue_url, attempts, last_error, created_at, updated_at, synced_at`

type SupportReportStore struct {
	db *sqlx.DB
}

func NewSupportReportStore(db *sqlx.DB) *SupportReportStore {
	return &SupportReportStore{db: db}
}

// Create inserts a pending report and fills in its id and timestamps.
func (s *SupportReportStore) Create(ctx context.Context, r *SupportReport) error {
	payload := r.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return s.db.QueryRowxContext(ctx, `
		INSERT INTO support_reports (public_ref, user_id, kind, title, payload)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, status, created_at, updated_at`,
		r.PublicRef, r.UserID, r.Kind, r.Title, string(payload),
	).Scan(&r.ID, &r.Status, &r.CreatedAt, &r.UpdatedAt)
}

// ListByUser returns the account's reports, newest first.
func (s *SupportReportStore) ListByUser(ctx context.Context, userID int) ([]SupportReport, error) {
	var out []SupportReport
	err := s.db.SelectContext(ctx, &out, `SELECT `+supportReportColumns+`
		FROM support_reports WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT 200`, userID)
	return out, err
}

// CountSince counts the account's reports created after since, pending ones
// included, and returns the oldest of them so the caller can tell the user
// when the window frees a slot.
func (s *SupportReportStore) CountSince(ctx context.Context, userID int, since time.Time) (int, time.Time, error) {
	var row struct {
		N      int          `db:"n"`
		Oldest sql.NullTime `db:"oldest"`
	}
	err := s.db.GetContext(ctx, &row, `
		SELECT COUNT(*) AS n, MIN(created_at) AS oldest
		FROM support_reports WHERE user_id = $1 AND created_at > $2`, userID, since)
	if err != nil {
		return 0, time.Time{}, err
	}
	return row.N, row.Oldest.Time, nil
}

// MarkSent records the GitHub issue and clears the payload: from now on the
// content lives on GitHub only.
func (s *SupportReportStore) MarkSent(ctx context.Context, id int, repo string, number int, url string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE support_reports
		SET status = 'open', repo = $2, issue_number = $3, issue_url = $4,
		    payload = '{}'::jsonb, last_error = NULL, attempts = attempts + 1,
		    updated_at = NOW(), synced_at = NOW()
		WHERE id = $1`, id, repo, number, url)
	return err
}

// MarkFailed counts a failed send attempt. msg must not contain user content.
func (s *SupportReportStore) MarkFailed(ctx context.Context, id int, msg string) error {
	if len(msg) > 300 {
		msg = msg[:300]
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE support_reports SET attempts = attempts + 1, last_error = $2, updated_at = NOW()
		WHERE id = $1`, id, msg)
	return err
}

// ListPending returns reports still waiting for their issue, with an
// exponential backoff (1, 2, 4… minutes) and a cap of 10 attempts.
func (s *SupportReportStore) ListPending(ctx context.Context, limit int) ([]SupportReport, error) {
	var out []SupportReport
	err := s.db.SelectContext(ctx, &out, `SELECT `+supportReportColumns+`
		FROM support_reports
		WHERE status = 'pending' AND attempts < 10
		  AND (attempts = 0 OR updated_at < NOW() - INTERVAL '1 minute' * POWER(2, attempts - 1))
		ORDER BY created_at LIMIT $1`, limit)
	return out, err
}

// ListOpenToSync returns open reports whose GitHub state was last read before
// olderThan.
func (s *SupportReportStore) ListOpenToSync(ctx context.Context, olderThan time.Time, limit int) ([]SupportReport, error) {
	var out []SupportReport
	err := s.db.SelectContext(ctx, &out, `SELECT `+supportReportColumns+`
		FROM support_reports
		WHERE status = 'open' AND (synced_at IS NULL OR synced_at < $1)
		ORDER BY synced_at NULLS FIRST LIMIT $2`, olderThan, limit)
	return out, err
}

// UpdateStatus stores the state read back from GitHub.
func (s *SupportReportStore) UpdateStatus(ctx context.Context, id int, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE support_reports
		SET status = $2::text, synced_at = NOW(),
		    updated_at = CASE WHEN status = $2::text THEN updated_at ELSE NOW() END
		WHERE id = $1`, id, status)
	return err
}

// ErrSyncLockBusy means another instance holds the sync lock.
var ErrSyncLockBusy = errors.New("support sync lock held by another instance")

// WithSyncLock runs fn while holding the cluster-wide advisory lock, on a
// dedicated connection so the unlock reaches the session that locked.
func (s *SupportReportStore) WithSyncLock(ctx context.Context, fn func() error) error {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var got bool
	if err := conn.GetContext(ctx, &got, `SELECT pg_try_advisory_lock($1)`, supportSyncLockKey); err != nil {
		return err
	}
	if !got {
		return ErrSyncLockBusy
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, supportSyncLockKey)
	return fn()
}
