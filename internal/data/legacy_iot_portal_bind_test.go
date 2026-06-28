package data

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestPortalMachineIDFromHashedPkey_NumericClientID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	store := NewLegacyIoTStore(sqlxDB)

	mock.ExpectQuery(`SELECT client_id FROM machines WHERE hashed_pkey`).
		WithArgs("hash123").
		WillReturnRows(sqlmock.NewRows([]string{"client_id"}).AddRow("4"))

	id, err := store.PortalMachineIDFromHashedPkey("hash123")
	if err != nil {
		t.Fatal(err)
	}
	if id != 4 {
		t.Fatalf("expected 4, got %d", id)
	}
}

func TestPortalMachineIDFromHashedPkey_InventoryFallback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	store := NewLegacyIoTStore(sqlxDB)

	mock.ExpectQuery(`SELECT client_id FROM machines WHERE hashed_pkey`).
		WithArgs("hashabc").
		WillReturnRows(sqlmock.NewRows([]string{"client_id"}).AddRow("UNKNOWN-abc"))

	mock.ExpectQuery(`SELECT inv_id FROM`).
		WithArgs("hashabc").
		WillReturnRows(sqlmock.NewRows([]string{"inv_id"}).AddRow(4))

	id, err := store.PortalMachineIDFromHashedPkey("hashabc")
	if err != nil {
		t.Fatal(err)
	}
	if id != 4 {
		t.Fatalf("expected 4, got %d", id)
	}
}

func TestBindMachineForPortalDelivery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	store := NewLegacyIoTStore(sqlxDB)

	mock.ExpectExec(`UPDATE machines SET is_active = true, client_id`).
		WithArgs("hashxyz", "4").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.BindMachineForPortalDelivery("hashxyz", 4); err != nil {
		t.Fatal(err)
	}
}
