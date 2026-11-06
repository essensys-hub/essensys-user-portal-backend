package identity

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/remember"
)

// NR-user-portal-backend-1 (feature session-30-jours-2026-11-001, #48).
//
// The 30 days of a remembered session run from the password entry. Using the
// session every day must never move its end, and the 31st day must be refused.
// sqlmock is strict: a refresh may execute exactly the reads and the
// last_used_at write set up below, so any statement that touched expires_at
// would be an unexpected call and fail the test. The same run is captured from
// the log to prove no token is ever written to it.
func TestNR_UserPortalBackend_1_RefreshNeverExtendsTheSessionNorLogsTokens(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	f := newRememberFixture(t, true)
	defer f.done()

	// Login with remember_me on day 0.
	expectLoginOK(f, t)
	var digest string
	expires := rememberTestEpoch.Add(remember.Lifetime)
	f.mock.ExpectBegin()
	f.mock.ExpectExec(`INSERT INTO remember_sessions`).
		WithArgs(1, captureArg{&digest}, expires, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	f.mock.ExpectExec(`UPDATE remember_sessions SET revoked_at`).WillReturnResult(sqlmock.NewResult(0, 0))
	f.mock.ExpectCommit()
	rec := postJSON(f.h.Login, "/api/auth/login", map[string]any{"email": "user@example.com", "password": "correct-password", "remember_me": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	token, _ := decode(t, rec)["remember_token"].(string)
	if token == "" {
		t.Fatal("no remember_token")
	}

	// Daily use for 29 days: always valid, always the same end date.
	for day := 1; day <= 29; day++ {
		*f.clock = rememberTestEpoch.Add(time.Duration(day) * 24 * time.Hour)
		f.mock.ExpectQuery(`SELECT \* FROM remember_sessions WHERE token_hash`).
			WithArgs(remember.Hash(token)).WillReturnRows(sessionRow(3, 1, remember.Hash(token), expires, nil))
		f.mock.ExpectQuery(`SELECT \* FROM users WHERE id = \$1`).WithArgs(1).WillReturnRows(rememberUserRow(1, nil))
		f.mock.ExpectExec(`^UPDATE remember_sessions SET last_used_at = \$2 WHERE id = \$1$`).
			WithArgs(3, *f.clock).WillReturnResult(sqlmock.NewResult(0, 1))
		rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": token})
		if rec.Code != http.StatusOK {
			t.Fatalf("day %d: refresh = %d, want 200", day, rec.Code)
		}
	}

	// Day 31: the same session is refused, however recently it was used.
	*f.clock = rememberTestEpoch.Add(31 * 24 * time.Hour)
	f.mock.ExpectQuery(`SELECT \* FROM remember_sessions WHERE token_hash`).
		WithArgs(remember.Hash(token)).WillReturnRows(sessionRow(3, 1, remember.Hash(token), expires, nil))
	rec = postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": token})
	if rec.Code != http.StatusUnauthorized || decode(t, rec)["error"] != "remember_invalid" {
		t.Fatalf("day 31: %d %s, want 401 remember_invalid", rec.Code, rec.Body.String())
	}

	// Signing out is logged too: still no token.
	f.mock.ExpectExec(`UPDATE remember_sessions SET revoked_at = \$2\s+WHERE token_hash`).
		WithArgs(remember.Hash(token), *f.clock).WillReturnResult(sqlmock.NewResult(0, 1))
	postJSON(f.h.Logout, "/api/auth/logout", map[string]string{"remember_token": token})

	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if out := logs.String(); strings.Contains(out, token) {
		t.Fatalf("the remember token must never be logged, found it in: %s", out)
	}
	if out := logs.String(); strings.Contains(out, "correct-password") {
		t.Fatal("the password must never be logged")
	}
}
