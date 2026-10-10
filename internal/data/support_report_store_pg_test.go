package data

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// TestSupportStore_RealPostgres runs the store against a real database when
// SUPPORT_PG_DSN is set (a throwaway instance with migration 015 applied).
// sqlmock cannot catch type mismatches such as []byte sent to a JSONB column.
func TestSupportStore_RealPostgres(t *testing.T) {
	dsn := os.Getenv("SUPPORT_PG_DSN")
	if dsn == "" {
		t.Skip("SUPPORT_PG_DSN not set")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	s := NewSupportReportStore(db)
	if _, err := db.Exec(`TRUNCATE support_reports`); err != nil {
		t.Fatal(err)
	}
	r := &SupportReport{PublicRef: "R-PGTEST01", UserID: 1, Kind: "bug", Title: "t", Payload: []byte(`{"actual":"x"}`)}
	if err := s.Create(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	n, oldest, err := s.CountSince(ctx, 1, time.Now().Add(-time.Hour))
	if err != nil || n != 1 || oldest.IsZero() {
		t.Fatalf("count n=%d oldest=%v err=%v", n, oldest, err)
	}
	pending, err := s.ListPending(ctx, 10)
	if err != nil || len(pending) != 1 || string(pending[0].Payload) != `{"actual": "x"}` {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	if err := s.MarkFailed(ctx, r.ID, "github: 502"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkSent(ctx, r.ID, "o/r", 4, "https://github.com/o/r/issues/4"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateStatus(ctx, r.ID, SupportStatusResolved); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListByUser(ctx, 1)
	if err != nil || len(got) != 1 || got[0].Status != "resolved" || string(got[0].Payload) != "{}" || got[0].Attempts != 2 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	open, err := s.ListOpenToSync(ctx, time.Now(), 10)
	if err != nil || len(open) != 0 {
		t.Fatalf("open=%v err=%v", open, err)
	}
	ran := false
	if err := s.WithSyncLock(ctx, func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("lock err=%v ran=%v", err, ran)
	}
}
