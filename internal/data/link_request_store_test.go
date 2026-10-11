package data

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestRevokeUserLinks(t *testing.T) {
	tests := []struct {
		name     string
		affected int64
	}{
		{name: "revokes approved links", affected: 2},
		{name: "nothing to revoke", affected: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			mock.ExpectExec(`UPDATE link_requests SET status = \$1, reviewed_by = \$2, reviewed_at = NOW\(\)\s+WHERE status = \$3\s+AND user_id = \(SELECT user_id FROM link_requests WHERE id = \$4\)`).
				WithArgs("revoked", "admin@essensys.fr", "approved", 7).
				WillReturnResult(sqlmock.NewResult(0, tt.affected))

			store := NewPortalStore(sqlx.NewDb(db, "sqlmock"))
			n, err := store.RevokeUserLinks(context.Background(), 7, "admin@essensys.fr")
			if err != nil {
				t.Fatalf("RevokeUserLinks: %v", err)
			}
			if n != tt.affected {
				t.Fatalf("affected = %d, want %d", n, tt.affected)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

func TestListLinkRequestsAdminHistoryIncludesRevoked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`WHERE lr.status IN \(\$2, \$3, \$4\)`).
		WithArgs(100, "approved", "rejected", "revoked").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	store := NewPortalStore(sqlx.NewDb(db, "sqlmock"))
	if _, err := store.ListLinkRequestsAdmin(context.Background(), "history", 100); err != nil {
		t.Fatalf("ListLinkRequestsAdmin: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
