package data

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func newResetStore(t *testing.T) (*PasswordResetStore, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	return NewPasswordResetStore(sqlx.NewDb(db, "sqlmock")), mock, func() { _ = db.Close() }
}

// The single-use guarantee rests entirely on the consume being one conditional
// UPDATE: Postgres serialises the row, so a second caller matches nothing.
// sqlmock cannot exercise real concurrency, so this asserts the guard clauses
// that make the database enforce it, and the race itself is covered by
// TestConsumeAndSetPassword_ConcurrentLoserGetsError below.
func TestConsumeAndSetPassword_GuardsAreConditional(t *testing.T) {
	store, mock, done := newResetStore(t)
	defer done()

	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WithArgs("digest", "1.2.3.4").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(12))
	mock.ExpectExec(`UPDATE users SET password_hash`).
		WithArgs("newhash", 12).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).
		WithArgs(12).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	userID, err := store.ConsumeAndSetPassword("digest", "newhash", "1.2.3.4")
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if userID != 12 {
		t.Fatalf("expected user 12, got %d", userID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConsumeAndSetPassword_ConcurrentLoserGetsError(t *testing.T) {
	store, mock, done := newResetStore(t)
	defer done()

	// The loser of the race matches zero rows and must not touch the password.
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WithArgs("digest", "1.2.3.4").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectRollback()

	_, err := store.ConsumeAndSetPassword("digest", "newhash", "1.2.3.4")
	if !errors.Is(err, ErrTokenNotConsumable) {
		t.Fatalf("expected ErrTokenNotConsumable, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConsumeAndSetPassword_RollsBackWhenPasswordWriteFails(t *testing.T) {
	store, mock, done := newResetStore(t)
	defer done()

	// A consumed token that never changed the password would lock the user out
	// for good, so the whole thing has to roll back.
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WithArgs("digest", nil).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(12))
	mock.ExpectExec(`UPDATE users SET password_hash`).
		WithArgs("newhash", 12).
		WillReturnError(errors.New("disk full"))
	mock.ExpectRollback()

	if _, err := store.ConsumeAndSetPassword("digest", "newhash", ""); err == nil {
		t.Fatal("expected an error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidateForUserSkipsSpentRows(t *testing.T) {
	store, mock, done := newResetStore(t)
	defer done()

	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).
		WithArgs(12).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.InvalidateForUser(12); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateStoresNullForMissingIP(t *testing.T) {
	store, mock, done := newResetStore(t)
	defer done()

	expires := time.Now().Add(time.Hour)
	mock.ExpectExec(`INSERT INTO password_reset_tokens`).
		WithArgs(12, "digest", expires, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.Create(12, "digest", expires, ""); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueInvalidatesEarlierTokensBeforeCreating(t *testing.T) {
	store, mock, done := newResetStore(t)
	defer done()

	// Order matters: if the new row were inserted first, the invalidate sweep
	// would immediately retire the token we just handed out.
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).
		WithArgs(12).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO password_reset_tokens`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`DELETE FROM password_reset_tokens`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	plain, expiresAt, err := store.Issue(12, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != 43 {
		t.Fatalf("expected a 43-char token, got %d chars", len(plain))
	}
	if strings.TrimSpace(plain) == "" {
		t.Fatal("issued an empty token")
	}
	if remaining := time.Until(expiresAt); remaining <= 0 || remaining > 61*time.Minute {
		t.Fatalf("unexpected TTL: %v", remaining)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueSurvivesPurgeFailure(t *testing.T) {
	store, mock, done := newResetStore(t)
	defer done()

	// Housekeeping must never cost a user their reset link.
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).
		WithArgs(12).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO password_reset_tokens`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`DELETE FROM password_reset_tokens`).
		WillReturnError(errors.New("lock timeout"))

	if _, _, err := store.Issue(12, ""); err != nil {
		t.Fatalf("purge failure must not fail Issue: %v", err)
	}
}
