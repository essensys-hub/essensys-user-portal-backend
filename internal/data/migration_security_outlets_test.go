package data

import (
	"os"
	"strings"
	"testing"
)

// security-outlets-2026-10-011 (#43): migrations run at every start, so the
// "Prises de sécurité" profile must be inserted once, on [440,440], with push.
func TestMigration016_profil_prises_de_securite_idempotent(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/016_security_outlets_sync_profile.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, want := range []string{
		"'Prises de sécurité'",
		"'[[440,440]]'::jsonb",
		"WHERE NOT EXISTS",
		"WHERE name = 'Prises de sécurité'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration 016 must contain %q", want)
		}
	}
	if strings.Contains(strings.ToUpper(sql), "UPDATE ") || strings.Contains(strings.ToUpper(sql), "DELETE ") {
		t.Fatal("migration 016 must not modify existing profiles")
	}
}
