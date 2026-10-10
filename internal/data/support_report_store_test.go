package data

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func newSupportStore(t *testing.T) (*SupportReportStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSupportReportStore(sqlx.NewDb(db, "sqlmock")), mock
}

func TestSupportCreate_ReturnsIDAndPendingStatus(t *testing.T) {
	store, mock := newSupportStore(t)
	now := time.Now()
	mock.ExpectQuery(`INSERT INTO support_reports`).
		WithArgs("R-ABC", 7, "bug", "Titre", `{"app":"web"}`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "created_at", "updated_at"}).AddRow(3, "pending", now, now))

	r := &SupportReport{PublicRef: "R-ABC", UserID: 7, Kind: "bug", Title: "Titre", Payload: []byte(`{"app":"web"}`)}
	if err := store.Create(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if r.ID != 3 || r.Status != SupportStatusPending {
		t.Fatalf("got id=%d status=%s", r.ID, r.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSupportCountSince_ReturnsOldest(t *testing.T) {
	store, mock := newSupportStore(t)
	oldest := time.Now().Add(-3 * time.Hour)
	since := time.Now().Add(-24 * time.Hour)
	mock.ExpectQuery(`SELECT COUNT\(\*\) AS n, MIN\(created_at\) AS oldest\s+FROM support_reports WHERE user_id = \$1 AND created_at > \$2`).
		WithArgs(7, since).
		WillReturnRows(sqlmock.NewRows([]string{"n", "oldest"}).AddRow(5, oldest))

	n, got, err := store.CountSince(context.Background(), 7, since)
	if err != nil || n != 5 || !got.Equal(oldest) {
		t.Fatalf("n=%d oldest=%v err=%v", n, got, err)
	}
}

// Once GitHub holds the content, the portal must not keep a second copy.
func TestSupportMarkSent_ClearsPayload(t *testing.T) {
	store, mock := newSupportStore(t)
	mock.ExpectExec(`UPDATE support_reports\s+SET status = 'open', repo = \$2, issue_number = \$3, issue_url = \$4,\s+payload = '\{\}'::jsonb`).
		WithArgs(3, "essensys-hub/essensys-support-site", 42, "https://github.com/x/42").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.MarkSent(context.Background(), 3, "essensys-hub/essensys-support-site", 42, "https://github.com/x/42"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSupportMarkFailed_TruncatesMessage(t *testing.T) {
	store, mock := newSupportStore(t)
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'x'
	}
	mock.ExpectExec(`UPDATE support_reports SET attempts = attempts \+ 1, last_error = \$2`).
		WithArgs(3, string(long[:300])).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.MarkFailed(context.Background(), 3, string(long)); err != nil {
		t.Fatal(err)
	}
}

func TestSupportListByUser_FiltersOnUser(t *testing.T) {
	store, mock := newSupportStore(t)
	mock.ExpectQuery(`FROM support_reports WHERE user_id = \$1 ORDER BY created_at DESC`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_ref", "user_id", "kind", "title", "payload", "status"}).
			AddRow(1, "R-1", 7, "bug", "t", []byte(`{}`), "open"))
	got, err := store.ListByUser(context.Background(), 7)
	if err != nil || len(got) != 1 || got[0].UserID != 7 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestSupportWithSyncLock_BusyDoesNotRun(t *testing.T) {
	store, mock := newSupportStore(t)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock`).WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(false))
	ran := false
	err := store.WithSyncLock(context.Background(), func() error { ran = true; return nil })
	if err != ErrSyncLockBusy || ran {
		t.Fatalf("err=%v ran=%v", err, ran)
	}
}

func TestSupportWithSyncLock_RunsAndUnlocks(t *testing.T) {
	store, mock := newSupportStore(t)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock`).WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(true))
	mock.ExpectExec(`SELECT pg_advisory_unlock`).WillReturnResult(sqlmock.NewResult(0, 0))
	ran := false
	if err := store.WithSyncLock(context.Background(), func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("err=%v ran=%v", err, ran)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
