package data

import (
	"os"
	"strings"
	"testing"
)

// sonnette-plages-push-2026-11-008 (#61): migration 022 is replayed at every
// start of the binary, so it must only create what is missing.
func TestMigration022_push_devices_idempotente(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/022_push_devices.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS push_devices",
		"platform TEXT NOT NULL",
		"token TEXT NOT NULL",
		"doorbell_notify BOOLEAN NOT NULL DEFAULT FALSE",
		"UNIQUE (platform, token)",
		"CREATE TABLE IF NOT EXISTS doorbell_rings",
		"PRIMARY KEY (gateway_id, ring_id)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration 022 must contain %q", want)
		}
	}
	up := strings.ToUpper(sql)
	for _, forbidden := range []string{"DROP ", "DELETE ", "UPDATE ", "TRUNCATE", "INSERT ", "ALTER "} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("migration 022 must not contain %q (replayed at each start)", forbidden)
		}
	}
	for _, line := range strings.Split(up, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "CREATE ") && !strings.Contains(line, "IF NOT EXISTS") {
			t.Fatalf("every CREATE must be guarded: %q", line)
		}
	}
}
