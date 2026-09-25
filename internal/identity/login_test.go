package identity

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// loginUserColumns mirrors domain.User's db tags, matching UserStore's
// `SELECT * FROM users WHERE email = $1`.
var loginUserColumns = []string{
	"id", "email", "password_hash", "role", "first_name", "last_name",
	"provider", "provider_id", "created_at", "last_login", "forbidden_at",
	"linked_machine_id", "linked_gateway_id", "linked_armoire_id",
	"password_change_required_at", "temp_password_expires_at", "temp_password_issued_by",
}

func loginUserRow(email, hash string, changeRequiredAt, tempExpiresAt *time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(loginUserColumns).AddRow(
		1, email, hash, domain.RoleUser, "First", "Last",
		domain.ProviderEmail, "", time.Now(), time.Now(), nil,
		nil, nil, nil,
		changeRequiredAt, tempExpiresAt, nil,
	)
}

func newLoginHandler(t *testing.T) (*Handlers, sqlmock.Sqlmock, func()) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(data.NewUserStore(sqlx.NewDb(db, "sqlmock")), nil, false)
	return h, mock, func() { _ = db.Close() }
}

func callLogin(h *Handlers, email, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(domain.LoginRequest{Email: email, Password: password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	return rec
}

func bcryptHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestLogin_OrdinaryAccountDoesNotRequireChange(t *testing.T) {
	h, mock, done := newLoginHandler(t)
	defer done()
	hash := bcryptHash(t, "correct-password")
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, nil, nil))
	mock.ExpectExec(`UPDATE users SET last_login`).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := callLogin(h, "user@example.com", "correct-password")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	if body["password_change_required"] != false {
		t.Fatalf("expected password_change_required=false, got %v", body["password_change_required"])
	}
	if body["token"] == "" || body["token"] == nil {
		t.Fatal("expected a token")
	}
}

func TestLogin_ValidNonExpiredTemporaryPassword(t *testing.T) {
	h, mock, done := newLoginHandler(t)
	defer done()
	hash := bcryptHash(t, "temp-password")
	changeRequiredAt := time.Now().Add(-time.Minute)
	expiresAt := time.Now().Add(71 * time.Hour)
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, &changeRequiredAt, &expiresAt))
	mock.ExpectExec(`UPDATE users SET last_login`).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := callLogin(h, "user@example.com", "temp-password")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	if body["password_change_required"] != true {
		t.Fatalf("expected password_change_required=true, got %v", body["password_change_required"])
	}
	if body["token"] == "" || body["token"] == nil {
		t.Fatal("expected a token even though a change is required — the lock is enforced server-side per request, not by withholding the token")
	}
}

func TestLogin_ExpiredTemporaryPasswordIsRejected(t *testing.T) {
	h, mock, done := newLoginHandler(t)
	defer done()
	hash := bcryptHash(t, "temp-password")
	changeRequiredAt := time.Now().Add(-73 * time.Hour)
	expiresAt := time.Now().Add(-time.Hour)
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, &changeRequiredAt, &expiresAt))

	rec := callLogin(h, "user@example.com", "temp-password")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	if body["error"] != "temporary_password_expired" {
		t.Fatalf(`expected error="temporary_password_expired", got %q`, body["error"])
	}
	// No UPDATE users SET last_login and no token: the mock has no
	// expectation for it, so a call would fail ExpectationsWereMet below —
	// but more directly, the response must carry no token.
	if _, hasToken := body["token"]; hasToken {
		t.Fatal("no token should be issued for an expired temporary password")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected extra DB call: %v", err)
	}
}

// A wrong password on an account whose temporary password happens to be
// expired must look identical to a wrong password on any other account: the
// bcrypt comparison fails first, so TempPasswordExpired is never reached and
// nothing about the account's temporary-password state leaks.
func TestLogin_WrongPasswordOnExpiredTempPasswordDoesNotLeakState(t *testing.T) {
	h, mock, done := newLoginHandler(t)
	defer done()
	hash := bcryptHash(t, "temp-password")
	changeRequiredAt := time.Now().Add(-73 * time.Hour)
	expiresAt := time.Now().Add(-time.Hour)
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", hash, &changeRequiredAt, &expiresAt))

	rec := callLogin(h, "user@example.com", "wrong-password")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "temporary_password_expired") {
		t.Fatalf("wrong password must not disclose temporary-password state: %s", body)
	}
}
