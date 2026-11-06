package data

import (
	"os"
	"strings"
	"testing"
)

// lumieres-temporisees-2026-11-003 (#53): the "Temporisations lumières" profile
// is inserted once, on the six keys, with pull and push, and never touches an
// existing profile.
func TestMigration018_profil_temporisations_lumieres_idempotent(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/018_lights_timers_sync_profile.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, want := range []string{
		"'Temporisations lumières'",
		"'[[521,521],[555,557],[561,562]]'::jsonb",
		"TRUE, TRUE, TRUE",
		"WHERE NOT EXISTS",
		"WHERE name = 'Temporisations lumières'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration 018 must contain %q", want)
		}
	}
	up := strings.ToUpper(sql)
	if strings.Contains(up, "UPDATE ") || strings.Contains(up, "DELETE ") {
		t.Fatal("migration 018 must not modify or delete existing profiles")
	}
	if strings.Count(up, "INSERT INTO") != 1 {
		t.Fatal("migration 018 must contain a single INSERT")
	}
}
