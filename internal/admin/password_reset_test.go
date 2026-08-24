package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

const adminEmail = "admin@essensys.fr"

func newResetAdmin(t *testing.T, withResets bool) (*Handlers, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	deps := Deps{Users: data.NewUserStore(sqlxDB)}
	if withResets {
		deps.Resets = data.NewPasswordResetStore(sqlxDB)
	}
	return NewHandlers(deps), mock, func() { _ = db.Close() }
}

func adminUserRows(role string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "email", "password_hash", "role", "first_name", "last_name", "provider", "provider_id", "created_at", "last_login", "forbidden_at"}).
		AddRow(1, adminEmail, "hash", role, "Admin", "", "email", "", time.Now(), time.Now(), nil)
}

func targetUserRows(forbiddenAt *time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "email", "password_hash", "role", "first_name", "last_name", "provider", "provider_id", "created_at", "last_login", "forbidden_at"}).
		AddRow(12, "emilienbieber67260@gmail.com", "hash", domain.RoleUser, "Emilien", "B", "email", "", time.Now(), time.Now(), forbiddenAt)
}

func callSendReset(h *Handlers, id, callerEmail string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+id+"/password-reset", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if callerEmail != "" {
		ctx = context.WithValue(ctx, middleware.UserEmailKey, callerEmail)
	}
	rec := httptest.NewRecorder()
	h.SendPasswordReset(rec, req.WithContext(ctx))
	return rec
}

func TestSendPasswordResetRequiresGlobalAdmin(t *testing.T) {
	t.Run("no caller in context is 401", func(t *testing.T) {
		h, mock, done := newResetAdmin(t, true)
		defer done()
		mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		if code := callSendReset(h, "12", "").Code; code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", code)
		}
	})

	t.Run("non-admin caller is 403", func(t *testing.T) {
		h, mock, done := newResetAdmin(t, true)
		defer done()
		mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
			WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleUser))

		if code := callSendReset(h, "12", adminEmail).Code; code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", code)
		}
	})
}

func TestSendPasswordResetUnknownUserIs404(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(999).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if code := callSendReset(h, "999", adminEmail).Code; code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestSendPasswordResetInvalidIDIs400(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))

	if code := callSendReset(h, "abc", adminEmail).Code; code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

// Emailing a reset to a banned account would quietly reopen it. Lifting the ban
// has to be a separate, deliberate decision.
func TestSendPasswordResetOnForbiddenAccountIs409(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	forbidden := time.Now()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(12).WillReturnRows(targetUserRows(&forbidden))

	rec := callSendReset(h, "12", adminEmail)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "account_forbidden") {
		t.Fatalf("expected 409 account_forbidden, got %d %s", rec.Code, rec.Body.String())
	}
	// No token may have been issued for a banned account.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSendPasswordResetWithoutStoreIs503(t *testing.T) {
	h, mock, done := newResetAdmin(t, false)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))

	if code := callSendReset(h, "12", adminEmail).Code; code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", code)
	}
}

// The token is issued even when delivery fails, so the response is 200 with
// email_sent=false rather than a 500: the action worked, the mail did not.
func TestSendPasswordResetReportsDeliveryFailureWithoutLeakingToken(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(12).WillReturnRows(targetUserRows(nil))
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).
		WithArgs(12).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO password_reset_tokens`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`DELETE FROM password_reset_tokens`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rec := callSendReset(h, "12", adminEmail)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	// h.templates is nil here, so delivery cannot succeed.
	if body["email_sent"] != false {
		t.Fatalf("expected email_sent=false, got %s", rec.Body.String())
	}
	if _, ok := body["expires_at"]; !ok {
		t.Fatalf("expected expires_at, got %s", rec.Body.String())
	}
	assertNoTokenLeak(t, rec.Body.String())
}

func assertNoTokenLeak(t *testing.T, body string) {
	t.Helper()
	for _, key := range []string{"token", "reset_url", "password", "hash", "digest"} {
		if strings.Contains(strings.ToLower(body), key) {
			t.Fatalf("admin response leaked %q: %s", key, body)
		}
	}
}

// Guards the invariant behind the whole design: nothing in the reset path may
// hand a reusable secret back over HTTP or put one in a log line.
func TestSendPasswordResetSourceNeverReturnsClearToken(t *testing.T) {
	raw, err := os.ReadFile("password_reset.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, forbidden := range []string{`"token": plain`, `"token":plain`, `Printf("`} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("source must not expose the clear-text token: %q", forbidden)
		}
	}
	if !strings.Contains(src, "BuildResetURL") {
		t.Fatal("expected the clear token to be used only to build the reset URL")
	}
}
