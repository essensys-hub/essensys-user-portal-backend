package data

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// noms-lumieres-2026-11-004 (#55): light_names store, with a mocked database.

func newNamesStoreMock(t *testing.T) (*PortalStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewPortalStore(sqlx.NewDb(db, "sqlmock")), mock
}

func TestListLightNames_parArmoire(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM light_names WHERE machine_id = $1`)).WithArgs(3).
		WillReturnRows(sqlmock.NewRows([]string{"light_id", "name", "version", "updated_at", "updated_by"}).
			AddRow("dressing", "WC 3", 2, at, "a@example.org").
			AddRow("wc1", nil, 1, at, "a@example.org"))
	got, err := s.ListLightNames(context.Background(), 3)
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if d := got["dressing"]; d.Name == nil || *d.Name != "WC 3" || d.Version != 2 || !d.UpdatedAt.Equal(at) || d.UpdatedBy != "a@example.org" {
		t.Fatalf("dressing: %+v", d)
	}
	if got["wc1"].Name != nil {
		t.Fatal("a reset name must come back as nil")
	}
}

func TestSetLightName_incrementeLaVersionEnBase(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 15, 30, 0, time.UTC)
	mock.ExpectQuery(`INSERT INTO light_names .* ON CONFLICT \(machine_id, light_id\) DO UPDATE .*light_names\.version \+ 1`).
		WithArgs(3, "dressing", "WC 3", sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows([]string{"light_id", "name", "version", "updated_at", "updated_by"}).AddRow("dressing", "WC 3", 4, at, "a@example.org"))
	n := "WC 3"
	got, err := s.SetLightName(context.Background(), 3, "dressing", &n, "a@example.org")
	if err != nil || got.Version != 4 {
		t.Fatalf("%+v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMergeLightNames_ecritExactementLaValeurFusionnee(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 20, 0, 0, time.UTC)
	mock.ExpectExec(`INSERT INTO light_names .* ON CONFLICT \(machine_id, light_id\) DO UPDATE SET name = EXCLUDED.name, version = EXCLUDED.version`).
		WithArgs(3, "wc1", "WC Haut", 5, at, "gw").WillReturnResult(sqlmock.NewResult(1, 1))
	n := "WC Haut"
	if err := s.PutLightNameExact(context.Background(), 3, "wc1", LightNameRow{Name: &n, Version: 5, UpdatedAt: at, UpdatedBy: "gw"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
