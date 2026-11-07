package data

import (
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// noms-lumieres-2026-11-004 (#55): migration 019 is replayed at every start of
// the binary, so it must only create what is missing and never touch data.
func TestMigration019_light_names_idempotente(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/019_light_names.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS light_names",
		"machine_id",
		"light_id",
		"name TEXT NULL",
		"version",
		"updated_at",
		"updated_by",
		"PRIMARY KEY (machine_id, light_id)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration 019 must contain %q", want)
		}
	}
	up := strings.ToUpper(sql)
	for _, forbidden := range []string{"DROP ", "DELETE ", "UPDATE ", "TRUNCATE", "INSERT "} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("migration 019 must not contain %q (replayed at each start)", forbidden)
		}
	}
	for _, line := range strings.Split(up, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "CREATE ") && !strings.Contains(line, "IF NOT EXISTS") {
			t.Fatalf("every CREATE must be guarded: %q", line)
		}
	}
}

// TestLightNames_RealPostgres replays migration 019 twice and exercises the
// store against a real database when LIGHT_NAMES_PG_DSN is set (a throwaway
// instance). sqlmock cannot check that the SQL itself is valid and idempotent.
func TestLightNames_RealPostgres(t *testing.T) {
	dsn := os.Getenv("LIGHT_NAMES_PG_DSN")
	if dsn == "" {
		t.Skip("LIGHT_NAMES_PG_DSN not set")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw, err := os.ReadFile("../../migrations/019_light_names.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("migration 019 run %d: %v", i, err)
		}
	}
	if _, err := db.Exec(`TRUNCATE light_names`); err != nil {
		t.Fatal(err)
	}
	s := NewPortalStore(db)
	ctx := t.Context()

	row, err := s.SetLightName(ctx, 3, "dressing", strPtr("WC 3"), "a@example.org")
	if err != nil || row.Version != 1 || row.Name == nil || *row.Name != "WC 3" {
		t.Fatalf("first write: %+v %v", row, err)
	}
	row, err = s.SetLightName(ctx, 3, "dressing", nil, "a@example.org")
	if err != nil || row.Version != 2 || row.Name != nil {
		t.Fatalf("reset must bump the version: %+v %v", row, err)
	}
	if _, err := db.Exec(string(raw)); err != nil {
		t.Fatalf("replay with data: %v", err)
	}
	all, err := s.ListLightNames(ctx, 3)
	if err != nil || len(all) != 1 || all["dressing"].Version != 2 {
		t.Fatalf("data lost by replay: %+v %v", all, err)
	}
	other, _ := s.ListLightNames(ctx, 4)
	if len(other) != 0 {
		t.Fatal("names are per cabinet")
	}
}

func strPtr(s string) *string { return &s }
