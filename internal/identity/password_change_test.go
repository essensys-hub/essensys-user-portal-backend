package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/jmoiron/sqlx"
)

func newChangePasswordHandler(t *testing.T) (*Handlers, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(data.NewUserStore(sqlx.NewDb(db, "sqlmock")), nil, false)
	return h, mock, func() { _ = db.Close() }
}

func callChangePassword(h *Handlers, email, current, newPw string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"current_password": current, "new_password": newPw})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password/change", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserEmailKey, email)
	rec := httptest.NewRecorder()
	h.ChangePassword(rec, req.WithContext(ctx))
	return rec
}

func expectClearPasswordChangeRequired(mock sqlmock.Sqlmock, userID int) {
	mock.ExpectExec(`(?s)UPDATE users\s+SET password_hash = \$1,\s+password_change_required_at = NULL,\s+temp_password_expires_at = NULL,\s+temp_password_issued_by = NULL\s+WHERE id = \$2`).
		WithArgs(sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestChangePassword_Succeeds(t *testing.T) {
	h, mock, done := newChangePasswordHandler(t)
	defer done()
	changeRequiredAt := time.Now().Add(-time.Minute)
	expiresAt := time.Now().Add(71 * time.Hour)
	hash := bcryptHash(t, "temp-password")
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, &changeRequiredAt, &expiresAt))
	expectClearPasswordChangeRequired(mock, 1)

	rec := callChangePassword(h, "user@example.com", "temp-password", "brand-new-password")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChangePassword_WrongCurrentPasswordIs401(t *testing.T) {
	h, mock, done := newChangePasswordHandler(t)
	defer done()
	hash := bcryptHash(t, "temp-password")
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, nil, nil))

	rec := callChangePassword(h, "user@example.com", "wrong-current", "brand-new-password")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChangePassword_WeakNewPasswordIs400(t *testing.T) {
	h, mock, done := newChangePasswordHandler(t)
	defer done()
	hash := bcryptHash(t, "temp-password")
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, nil, nil))

	rec := callChangePassword(h, "user@example.com", "temp-password", "short")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChangePassword_SameAsCurrentIs400(t *testing.T) {
	h, mock, done := newChangePasswordHandler(t)
	defer done()
	hash := bcryptHash(t, "temp-password")
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, nil, nil))

	rec := callChangePassword(h, "user@example.com", "temp-password", "temp-password")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestChangePasswordRateLimit_BlocksAfterLimit covers task 5.3's 429 case
// directly against the route middleware, without needing a full router
// mount.
func TestChangePasswordRateLimit_BlocksAfterLimit(t *testing.T) {
	limiter := middleware.NewRateLimiter(1, time.Hour)
	called := 0
	h := changePasswordRateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	doRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password/change", nil)
		req.RemoteAddr = "203.0.113.7:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := doRequest(); rec.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec := doRequest(); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if called != 1 {
		t.Fatalf("handler ran %d times, want exactly 1", called)
	}
}
