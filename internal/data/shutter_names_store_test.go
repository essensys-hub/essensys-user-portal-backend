package data

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// noms-volets-2026-11-005 (#57): shutter_names store, with a mocked database.

func TestListShutterNames_parArmoire(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM shutter_names WHERE machine_id = $1`)).WithArgs(3).
		WillReturnRows(sqlmock.NewRows([]string{"shutter_id", "name", "version", "updated_at", "updated_by"}).
			AddRow("volet1salon", "Volet baie vitrée", 2, at, "a@example.org").
			AddRow("store", nil, 1, at, "a@example.org"))
	got, err := s.ListShutterNames(context.Background(), 3)
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if d := got["volet1salon"]; d.Name == nil || *d.Name != "Volet baie vitrée" || d.Version != 2 || !d.UpdatedAt.Equal(at) || d.UpdatedBy != "a@example.org" {
		t.Fatalf("dressing: %+v", d)
	}
	if got["store"].Name != nil {
		t.Fatal("a reset name must come back as nil")
	}
}

func TestSetShutterName_incrementeLaVersionEnBase(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 15, 30, 0, time.UTC)
	mock.ExpectQuery(`INSERT INTO shutter_names .* ON CONFLICT \(machine_id, shutter_id\) DO UPDATE .*shutter_names\.version \+ 1`).
		WithArgs(3, "volet1salon", "Volet baie vitrée", sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows([]string{"shutter_id", "name", "version", "updated_at", "updated_by"}).AddRow("volet1salon", "Volet baie vitrée", 4, at, "a@example.org"))
	n := "Volet baie vitrée"
	got, err := s.SetShutterName(context.Background(), 3, "volet1salon", &n, "a@example.org")
	if err != nil || got.Version != 4 {
		t.Fatalf("%+v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMergeShutterNames_ecritExactementLaValeurFusionnee(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 20, 0, 0, time.UTC)
	mock.ExpectExec(`INSERT INTO shutter_names .* ON CONFLICT \(machine_id, shutter_id\) DO UPDATE SET name = EXCLUDED.name, version = EXCLUDED.version`).
		WithArgs(3, "store", "Store terrasse", 5, at, "gw").WillReturnResult(sqlmock.NewResult(1, 1))
	n := "Store terrasse"
	if err := s.PutShutterNameExact(context.Background(), 3, "store", ShutterNameRow{Name: &n, Version: 5, UpdatedAt: at, UpdatedBy: "gw"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
