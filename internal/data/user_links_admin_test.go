package data

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// NR-essensys-user-portal-backend-66 (#66) : une affectation admin éligible
// doit enregistrer une liaison approuvée, sinon le portail reste fermé.
func TestNR66_AdminAssignmentRecordsApprovedLink(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	machine, gw, armoire := 19, "gw-essensys-gateway", 15

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE users SET linked_machine_id`).
		WithArgs(&machine, &gw, &armoire, 14).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO link_requests .*WHERE NOT EXISTS`).
		WithArgs(14, "machine-19", sqlmock.AnyArg(), "approved", "admin@example.com").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := NewUserStore(sqlx.NewDb(db, "sqlmock"))
	if err := store.UpdateUserLinksByAdmin(14, &machine, &gw, &armoire, "admin@example.com"); err != nil {
		t.Fatalf("UpdateUserLinksByAdmin: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// Retirer les liens révoque la liaison approuvée.
func TestNR66_AdminUnassignRevokesApprovedLink(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE users SET linked_machine_id`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE link_requests SET status`).
		WithArgs("revoked", "admin@example.com", 14, "approved").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	store := NewUserStore(sqlx.NewDb(db, "sqlmock"))
	if err := store.UpdateUserLinksByAdmin(14, nil, nil, nil, "admin@example.com"); err != nil {
		t.Fatalf("UpdateUserLinksByAdmin: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
