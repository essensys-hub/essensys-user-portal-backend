package data

import (
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// noms-volets-2026-11-005 (#57): migration 020 is replayed at every start of
// the binary, so it must only create what is missing and never touch data.
func TestMigration020_shutter_names_idempotente(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/020_shutter_names.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS shutter_names",
		"machine_id",
		"shutter_id",
		"name TEXT NULL",
		"version",
		"updated_at",
		"updated_by",
		"PRIMARY KEY (machine_id, shutter_id)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration 020 must contain %q", want)
		}
	}
	up := strings.ToUpper(sql)
	for _, forbidden := range []string{"DROP ", "DELETE ", "UPDATE ", "TRUNCATE", "INSERT "} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("migration 020 must not contain %q (replayed at each start)", forbidden)
		}
	}
	for _, line := range strings.Split(sql, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") && strings.Contains(line, "light_names") {
			t.Fatalf("migration 020 must not touch light_names: %q", line)
		}
	}
	for _, line := range strings.Split(up, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "CREATE ") && !strings.Contains(line, "IF NOT EXISTS") {
			t.Fatalf("every CREATE must be guarded: %q", line)
		}
	}
}

// TestShutterNames_RealPostgres replays migration 020 twice and exercises the
// store against a real database when LIGHT_NAMES_PG_DSN is set (a throwaway
// instance). sqlmock cannot check that the SQL itself is valid and idempotent.
func TestShutterNames_RealPostgres(t *testing.T) {
	dsn := os.Getenv("LIGHT_NAMES_PG_DSN")
	if dsn == "" {
		t.Skip("LIGHT_NAMES_PG_DSN not set")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw, err := os.ReadFile("../../migrations/020_shutter_names.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("migration 020 run %d: %v", i, err)
		}
	}
	raw19, err := os.ReadFile("../../migrations/019_light_names.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(raw19)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`TRUNCATE light_names`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO light_names (machine_id, light_id, name, version) VALUES (3, 'dressing', 'WC 3', 2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`TRUNCATE shutter_names`); err != nil {
		t.Fatal(err)
	}
	s := NewPortalStore(db)
	ctx := t.Context()

	row, err := s.SetShutterName(ctx, 3, "volet1salon", strPtr("Volet baie vitrée"), "a@example.org")
	if err != nil || row.Version != 1 || row.Name == nil || *row.Name != "Volet baie vitrée" {
		t.Fatalf("first write: %+v %v", row, err)
	}
	row, err = s.SetShutterName(ctx, 3, "volet1salon", nil, "a@example.org")
	if err != nil || row.Version != 2 || row.Name != nil {
		t.Fatalf("reset must bump the version: %+v %v", row, err)
	}
	if _, err := db.Exec(string(raw)); err != nil {
		t.Fatalf("replay with data: %v", err)
	}
	all, err := s.ListShutterNames(ctx, 3)
	if err != nil || len(all) != 1 || all["volet1salon"].Version != 2 {
		t.Fatalf("data lost by replay: %+v %v", all, err)
	}
	var lights int
	if err := db.Get(&lights, `SELECT COUNT(*) FROM light_names WHERE name = 'WC 3' AND version = 2`); err != nil || lights != 1 {
		t.Fatalf("light_names must be unchanged: %d %v", lights, err)
	}
	other, _ := s.ListShutterNames(ctx, 4)
	if len(other) != 0 {
		t.Fatal("names are per cabinet")
	}
}
