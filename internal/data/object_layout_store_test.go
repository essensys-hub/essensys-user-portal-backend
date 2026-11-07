package data

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

// affichage-objets-2026-11-006 (#59) : migration 021 et magasin object_layout.

var layoutCols = []string{"order_json", "disabled_json", "version", "updated_at", "updated_by"}

func TestMigration021_object_layout_idempotente(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/021_object_layout.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS object_layout",
		"machine_id INT NOT NULL",
		"kind TEXT NOT NULL",
		"order_json TEXT NOT NULL DEFAULT '[]'",
		"disabled_json TEXT NOT NULL DEFAULT '[]'",
		"version INT NOT NULL DEFAULT 0",
		"updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
		"updated_by TEXT NOT NULL DEFAULT ''",
		"PRIMARY KEY (machine_id, kind)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("la migration 021 doit contenir %q", want)
		}
	}
	up := strings.ToUpper(sql)
	for _, forbidden := range []string{"DROP ", "DELETE ", "UPDATE ", "TRUNCATE", "INSERT ", "ALTER "} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("la migration 021 ne doit pas contenir %q (rejouée à chaque démarrage)", forbidden)
		}
	}
	for _, line := range strings.Split(sql, "\n") {
		code := strings.TrimSpace(line)
		if !strings.HasPrefix(code, "--") && (strings.Contains(code, "light_names") || strings.Contains(code, "shutter_names")) {
			t.Fatalf("la migration 021 ne touche pas aux tables de noms : %q", line)
		}
	}
	for _, line := range strings.Split(up, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "CREATE ") && !strings.Contains(line, "IF NOT EXISTS") {
			t.Fatalf("chaque CREATE doit être gardé : %q", line)
		}
	}
}

// Rejoue la migration deux fois sur une vraie base quand LIGHT_NAMES_PG_DSN est
// défini (instance jetable) : sqlmock ne vérifie pas le SQL lui-même.
func TestObjectLayout_RealPostgres(t *testing.T) {
	dsn := os.Getenv("LIGHT_NAMES_PG_DSN")
	if dsn == "" {
		t.Skip("LIGHT_NAMES_PG_DSN non défini")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, f := range []string{"019_light_names.sql", "020_shutter_names.sql"} {
		raw, err := os.ReadFile("../../migrations/" + f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`TRUNCATE light_names, shutter_names`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO light_names (machine_id, light_id, name, version) VALUES (3, 'dressing', 'WC 3', 2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO shutter_names (machine_id, shutter_id, name, version) VALUES (3, 'store', 'Banne', 1)`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../migrations/021_object_layout.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("migration 021, exécution %d : %v", i, err)
		}
	}
	if _, err := db.Exec(`TRUNCATE object_layout`); err != nil {
		t.Fatal(err)
	}
	s := NewPortalStore(db)
	ctx := t.Context()
	row, err := s.SetObjectLayout(ctx, 3, domain.LayoutLights, []string{"salon"}, []string{"wc2"}, "a@example.org")
	if err != nil || row.Version != 1 {
		t.Fatalf("première écriture : %+v %v", row, err)
	}
	row, err = s.SetObjectLayout(ctx, 3, domain.LayoutLights, []string{}, []string{}, "a@example.org")
	if err != nil || row.Version != 2 || len(row.Order) != 0 {
		t.Fatalf("reset : %+v %v", row, err)
	}
	if _, err := db.Exec(string(raw)); err != nil {
		t.Fatalf("rejeu avec données : %v", err)
	}
	if got, has, err := s.GetObjectLayout(ctx, 3, domain.LayoutLights); err != nil || !has || got.Version != 2 {
		t.Fatalf("donnée perdue au rejeu : %+v %v %v", got, has, err)
	}
	if _, has, _ := s.GetObjectLayout(ctx, 3, domain.LayoutShutters); has {
		t.Fatal("les genres sont indépendants")
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM light_names WHERE name = 'WC 3' AND version = 2`); err != nil || n != 1 {
		t.Fatalf("light_names doit être inchangée : %d %v", n, err)
	}
	if err := db.Get(&n, `SELECT COUNT(*) FROM shutter_names WHERE name = 'Banne' AND version = 1`); err != nil || n != 1 {
		t.Fatalf("shutter_names doit être inchangée : %d %v", n, err)
	}
}

func TestGetObjectLayout_defautSansLigne(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM object_layout WHERE machine_id = $1 AND kind = $2`)).WithArgs(3, "lights").
		WillReturnRows(sqlmock.NewRows(layoutCols))
	got, has, err := s.GetObjectLayout(context.Background(), 3, domain.LayoutLights)
	if err != nil || has {
		t.Fatalf("%+v %v %v", got, has, err)
	}
	if got.Version != 0 || !got.UpdatedAt.IsZero() || got.Order == nil || got.Disabled == nil || len(got.Order) != 0 {
		t.Fatalf("le défaut attendu : %+v", got)
	}
}

func TestGetObjectLayout_litLeDocument(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	mock.ExpectQuery(`FROM object_layout WHERE machine_id`).WithArgs(3, "shutters").
		WillReturnRows(sqlmock.NewRows(layoutCols).AddRow(`["store","volet1salon"]`, `["voletbureau"]`, 3, at, "a@example.org"))
	got, has, err := s.GetObjectLayout(context.Background(), 3, domain.LayoutShutters)
	if err != nil || !has {
		t.Fatalf("%+v %v %v", got, has, err)
	}
	if len(got.Order) != 2 || got.Order[0] != "store" || got.Disabled[0] != "voletbureau" || got.Version != 3 || !got.UpdatedAt.Equal(at) || got.UpdatedBy != "a@example.org" {
		t.Fatalf("%+v", got)
	}
}

func TestSetObjectLayout_ecritVersionNPlus1(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 15, 30, 0, time.UTC)
	mock.ExpectQuery(`INSERT INTO object_layout .* ON CONFLICT \(machine_id, kind\) DO UPDATE .*object_layout\.version \+ 1`).
		WithArgs(3, "lights", `["salon","entree"]`, `["wc2"]`, sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(layoutCols).AddRow(`["salon","entree"]`, `["wc2"]`, 4, at, "a@example.org"))
	got, err := s.SetObjectLayout(context.Background(), 3, domain.LayoutLights, []string{"salon", "entree"}, []string{"wc2"}, "a@example.org")
	if err != nil || got.Version != 4 || len(got.Order) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPutObjectLayoutExact_ecritLaValeurFusionnee(t *testing.T) {
	s, mock := newNamesStoreMock(t)
	at := time.Date(2026, 11, 7, 10, 20, 0, 0, time.UTC)
	mock.ExpectExec(`INSERT INTO object_layout .* ON CONFLICT \(machine_id, kind\) DO UPDATE SET order_json = EXCLUDED.order_json, disabled_json = EXCLUDED.disabled_json, version = EXCLUDED.version`).
		WithArgs(3, "shutters", `["store"]`, `[]`, 5, at, "gw").WillReturnResult(sqlmock.NewResult(1, 1))
	err := s.PutObjectLayoutExact(context.Background(), 3, domain.LayoutShutters,
		domain.ObjectLayout{Order: []string{"store"}, Disabled: []string{}, Version: 5, UpdatedAt: at, UpdatedBy: "gw"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
