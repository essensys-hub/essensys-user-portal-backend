package data

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/remember"
	"github.com/jmoiron/sqlx"
)

func newRememberStore(t *testing.T) (*RememberSessionStore, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	return NewRememberSessionStore(sqlx.NewDb(db, "sqlmock")), mock, func() { _ = db.Close() }
}

var rememberCols = []string{"id", "user_id", "token_hash", "created_at", "expires_at", "last_used_at", "revoked_at", "user_agent"}

func TestRememberCreate_StoresOnlyTheDigestAndCapsAtTen(t *testing.T) {
	store, mock, done := newRememberStore(t)
	defer done()
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	expires := now.Add(remember.Lifetime)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO remember_sessions \(user_id, token_hash, expires_at, user_agent\)`).
		WithArgs(7, "digest-64", expires, "Essensys/2.4 Android").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// The cap: everything past the ten most recent active sessions is revoked.
	mock.ExpectExec(`UPDATE remember_sessions SET revoked_at = \$2\s+WHERE id IN`).
		WithArgs(7, now, remember.MaxSessionsPerUser).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := store.Create(7, "digest-64", expires, "Essensys/2.4 Android", now); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRememberCreate_StoresNullForMissingUserAgent(t *testing.T) {
	store, mock, done := newRememberStore(t)
	defer done()
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO remember_sessions`).
		WithArgs(7, "d", now, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE remember_sessions SET revoked_at`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := store.Create(7, "d", now, "", now); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRememberCreate_RollsBackWhenTheCapFails(t *testing.T) {
	store, mock, done := newRememberStore(t)
	defer done()
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO remember_sessions`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE remember_sessions SET revoked_at`).WillReturnError(errors.New("boom"))
	mock.ExpectRollback()
	if err := store.Create(7, "d", now, "", now); err == nil {
		t.Fatal("expected the failure to surface")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRememberGetByHash(t *testing.T) {
	store, mock, done := newRememberStore(t)
	defer done()
	created := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT \* FROM remember_sessions WHERE token_hash = \$1`).
		WithArgs("known").
		WillReturnRows(sqlmock.NewRows(rememberCols).AddRow(3, 7, "known", created, created.Add(remember.Lifetime), nil, nil, nil))
	got, err := store.GetByHash("known")
	if err != nil || got == nil || got.ID != 3 || got.UserID != 7 {
		t.Fatalf("got %+v err=%v", got, err)
	}

	mock.ExpectQuery(`SELECT \* FROM remember_sessions WHERE token_hash = \$1`).
		WithArgs("unknown").
		WillReturnRows(sqlmock.NewRows(rememberCols))
	got, err = store.GetByHash("unknown")
	if err != nil || got != nil {
		t.Fatalf("unknown digest must give nil, nil; got %+v err=%v", got, err)
	}
}

// Touch must be the only statement that runs on a renewal and it must never
// write expires_at: that is what keeps the 30 days fixed.
func TestRememberTouch_NeverWritesExpiresAt(t *testing.T) {
	store, mock, done := newRememberStore(t)
	defer done()
	now := time.Now()
	mock.ExpectExec(`^UPDATE remember_sessions SET last_used_at = \$2 WHERE id = \$1$`).
		WithArgs(3, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.Touch(3, now); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile("remember_session_store.go")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?i)SET\s+[^;]*expires_at\s*=`).Match(src) {
		t.Fatal("no statement of the store may assign expires_at after creation")
	}
}

func TestRememberRevokeByHash(t *testing.T) {
	store, mock, done := newRememberStore(t)
	defer done()
	now := time.Now()
	mock.ExpectExec(`UPDATE remember_sessions SET revoked_at = \$2\s+WHERE token_hash = \$1 AND revoked_at IS NULL`).
		WithArgs("digest", now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.RevokeByHash("digest", now); err != nil {
		t.Fatalf("revoking an unknown or already revoked digest must not fail: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Migrations replay at every start (store.RunMigrations), so each statement of
// 017 must be idempotent; the clear token must have no column to live in.
func TestMigration017_idempotent_and_digest_only(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/017_remember_sessions.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	var stmts []string
	for _, line := range strings.Split(sql, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		stmts = append(stmts, line)
	}
	code := strings.ToUpper(strings.Join(stmts, "\n"))
	if strings.Count(code, "CREATE TABLE") != strings.Count(code, "CREATE TABLE IF NOT EXISTS") {
		t.Fatal("every CREATE TABLE of 017 must be IF NOT EXISTS")
	}
	if strings.Count(code, "CREATE INDEX") != strings.Count(code, "CREATE INDEX IF NOT EXISTS") {
		t.Fatal("every CREATE INDEX of 017 must be IF NOT EXISTS")
	}
	for _, forbidden := range []string{"DROP ", "DELETE FROM", "TRUNCATE ", "ALTER TABLE USERS"} {
		if strings.Contains(code, forbidden) {
			t.Fatalf("017 must not contain %q", forbidden)
		}
	}
	for _, want := range []string{"TOKEN_HASH CHAR(64) NOT NULL UNIQUE", "ON DELETE CASCADE", "EXPIRES_AT   TIMESTAMPTZ NOT NULL"} {
		if !strings.Contains(strings.Join(strings.Fields(code), " "), strings.Join(strings.Fields(want), " ")) {
			t.Fatalf("017 must contain %q", want)
		}
	}
	if regexp.MustCompile(`(?m)^\s+TOKEN\s`).MatchString(code) {
		t.Fatal("017 must not have a clear-text token column")
	}
}
