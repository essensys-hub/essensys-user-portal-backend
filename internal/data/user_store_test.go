package data

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// EnsureTableExists is re-run on every boot (there is no migration-tracking
// table), so it must stay the source of truth for the users schema alongside
// migrations/014_temporary_password.sql. This pins the three columns that
// migration adds, the way forbidden_at is already pinned by convention.
func TestUserStoreEnsureTableExists_AddsTemporaryPasswordColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`(?s)CREATE TABLE IF NOT EXISTS users.*` +
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS password_change_required_at TIMESTAMPTZ NULL.*` +
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS temp_password_expires_at TIMESTAMPTZ NULL.*` +
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS temp_password_issued_by INT NULL REFERENCES users\(id\) ON DELETE SET NULL`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store := NewUserStore(sqlx.NewDb(db, "sqlmock"))
	if err := store.EnsureTableExists(); err != nil {
		t.Fatalf("EnsureTableExists: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSetTemporaryPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	expiresAt := time.Now().Add(72 * time.Hour)

	mock.ExpectExec(`(?s)UPDATE users\s+SET password_hash = \$1,\s+password_change_required_at = NOW\(\),\s+temp_password_expires_at = \$2,\s+temp_password_issued_by = \$3\s+WHERE id = \$4`).
		WithArgs("newhash", expiresAt, 7, 42).
		WillReturnResult(sqlmock.NewResult(0, 1))

	store := NewUserStore(sqlx.NewDb(db, "sqlmock"))
	if err := store.SetTemporaryPassword(42, "newhash", expiresAt, 7); err != nil {
		t.Fatalf("SetTemporaryPassword: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestClearPasswordChangeRequired(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`(?s)UPDATE users\s+SET password_hash = \$1,\s+password_change_required_at = NULL,\s+temp_password_expires_at = NULL,\s+temp_password_issued_by = NULL\s+WHERE id = \$2`).
		WithArgs("newhash", 42).
		WillReturnResult(sqlmock.NewResult(0, 1))

	store := NewUserStore(sqlx.NewDb(db, "sqlmock"))
	if err := store.ClearPasswordChangeRequired(42, "newhash"); err != nil {
		t.Fatalf("ClearPasswordChangeRequired: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
