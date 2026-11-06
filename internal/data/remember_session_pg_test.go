package data

import (
	"os"
	"testing"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/remember"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// TestRememberSessions_RealPostgres replays migration 017 twice and exercises
// the store against a real database when REMEMBER_PG_DSN is set (a throwaway
// instance). sqlmock cannot check that the SQL itself is valid and idempotent.
func TestRememberSessions_RealPostgres(t *testing.T) {
	dsn := os.Getenv("REMEMBER_PG_DSN")
	if dsn == "" {
		t.Skip("REMEMBER_PG_DSN not set")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	users := NewUserStore(db)
	if err := users.EnsureTableExists(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../migrations/017_remember_sessions.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("migration 017 run %d: %v", i, err)
		}
	}
	if _, err := db.Exec(`TRUNCATE remember_sessions`); err != nil {
		t.Fatal(err)
	}
	var userID int
	if err := db.Get(&userID, `INSERT INTO users (email, password_hash) VALUES ($1, 'old') ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id`, "remember-pg@example.test"); err != nil {
		t.Fatal(err)
	}

	store := NewRememberSessionStore(db)
	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < remember.MaxSessionsPerUser+1; i++ {
		plain, digest, _ := remember.Generate()
		_ = plain
		if err := store.Create(userID, digest, base.Add(remember.Lifetime), "pg-test", base); err != nil {
			t.Fatalf("create #%d: %v", i, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	var active int
	if err := db.Get(&active, `SELECT count(*) FROM remember_sessions WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		t.Fatal(err)
	}
	if active != remember.MaxSessionsPerUser {
		t.Fatalf("11th session must leave exactly %d active, got %d", remember.MaxSessionsPerUser, active)
	}
	var oldestRevoked bool
	if err := db.Get(&oldestRevoked, `SELECT revoked_at IS NOT NULL FROM remember_sessions WHERE user_id = $1 ORDER BY created_at ASC, id ASC LIMIT 1`, userID); err != nil || !oldestRevoked {
		t.Fatalf("the oldest session must be the revoked one (revoked=%v err=%v)", oldestRevoked, err)
	}

	_, digest, _ := remember.Generate()
	if err := store.Create(userID, digest, base.Add(remember.Lifetime), "", base); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByHash(digest)
	if err != nil || got == nil || !got.Usable(base.Add(12*24*time.Hour)) || got.Usable(base.Add(remember.Lifetime+time.Minute)) {
		t.Fatalf("lookup/usability wrong: %+v err=%v", got, err)
	}
	before := got.ExpiresAt
	if err := store.Touch(got.ID, base.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	again, _ := store.GetByHash(digest)
	if !again.ExpiresAt.Equal(before) || again.LastUsedAt == nil {
		t.Fatalf("Touch must record last_used_at and leave expires_at alone: %+v", again)
	}
	if err := store.RevokeByHash(digest, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if again, _ = store.GetByHash(digest); again.Usable(base.Add(2 * time.Hour)) {
		t.Fatal("a revoked session must not be usable")
	}

	// A password write ends every remembered session of the account, atomically.
	if err := users.ClearPasswordChangeRequired(userID, "new-hash"); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&active, `SELECT count(*) FROM remember_sessions WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil || active != 0 {
		t.Fatalf("a password change must revoke all sessions, %d still active (err=%v)", active, err)
	}
}
