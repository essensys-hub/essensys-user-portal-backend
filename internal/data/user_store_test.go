package data

import (
	"testing"

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
