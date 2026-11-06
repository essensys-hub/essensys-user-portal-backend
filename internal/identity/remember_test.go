package identity

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/remember"
	"github.com/jmoiron/sqlx"
)

var rememberTestEpoch = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

var rememberRowColumns = []string{"id", "user_id", "token_hash", "created_at", "expires_at", "last_used_at", "revoked_at", "user_agent"}

// captureArg records the digest the handler hands to the store, so a test can
// prove it is the SHA-256 of the token returned to the app.
type captureArg struct{ got *string }

func (c captureArg) Match(v driver.Value) bool {
	s, ok := v.(string)
	if ok {
		*c.got = s
	}
	return ok
}

type rememberFixture struct {
	h     *Handlers
	mock  sqlmock.Sqlmock
	clock *time.Time
	done  func()
}

func newRememberFixture(t *testing.T, withRemember bool) *rememberFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	sx := sqlx.NewDb(db, "sqlmock")
	clock := rememberTestEpoch
	opts := []Option{}
	if withRemember {
		opts = append(opts, WithRememberSessions(data.NewRememberSessionStore(sx)))
	}
	h := NewHandlers(data.NewUserStore(sx), nil, false, opts...)
	h.now = func() time.Time { return clock }
	return &rememberFixture{h: h, mock: mock, clock: &clock, done: func() { _ = db.Close() }}
}

func rememberUserRow(id int, forbiddenAt *time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(loginUserColumns).AddRow(
		id, "user@example.com", "hash", domain.RoleUser, "First", "Last",
		domain.ProviderEmail, "", time.Now(), time.Now(), forbiddenAt,
		nil, nil, nil,
		nil, nil, nil,
	)
}

func sessionRow(id, userID int, hash string, expires time.Time, revoked *time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(rememberRowColumns).AddRow(id, userID, hash, rememberTestEpoch, expires, nil, revoked, nil)
}

func postJSON(h http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return m
}

// --- 1.2 login ---------------------------------------------------------

func expectLoginOK(f *rememberFixture, t *testing.T) {
	t.Helper()
	f.mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", bcryptHash(t, "correct-password"), nil, nil))
	f.mock.ExpectExec(`UPDATE users SET last_login`).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestLogin_RememberMe_ReturnsAThirtyDaySession(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	expectLoginOK(f, t)
	var digest string
	expires := f.clock.Add(remember.Lifetime)
	f.mock.ExpectBegin()
	f.mock.ExpectExec(`INSERT INTO remember_sessions`).
		WithArgs(1, captureArg{&digest}, expires, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	f.mock.ExpectExec(`UPDATE remember_sessions SET revoked_at = \$2\s+WHERE id IN`).
		WithArgs(1, *f.clock, remember.MaxSessionsPerUser).
		WillReturnResult(sqlmock.NewResult(0, 0))
	f.mock.ExpectCommit()

	rec := postJSON(f.h.Login, "/api/auth/login", map[string]any{"email": "user@example.com", "password": "correct-password", "remember_me": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	tok, _ := body["remember_token"].(string)
	if tok == "" || body["token"] == "" {
		t.Fatalf("expected both token and remember_token, got %v", body)
	}
	if body["remember_expires_at"] != expires.Format(time.RFC3339) {
		t.Fatalf("remember_expires_at = %v, want %s (login + 30 days)", body["remember_expires_at"], expires.Format(time.RFC3339))
	}
	if digest != remember.Hash(tok) {
		t.Fatal("the store must receive only the SHA-256 digest of the returned token")
	}
	if strings.Contains(digest, tok) {
		t.Fatal("the clear token must not reach the store")
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogin_WithoutRememberMe_ResponseIsUnchanged(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	expectLoginOK(f, t)

	rec := postJSON(f.h.Login, "/api/auth/login", map[string]any{"email": "user@example.com", "password": "correct-password"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := decode(t, rec)
	if _, ok := body["remember_token"]; ok {
		t.Fatal("no remember_token without remember_me")
	}
	if _, ok := body["remember_expires_at"]; ok {
		t.Fatal("no remember_expires_at without remember_me")
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err) // a remember INSERT was not expected
	}
}

func TestLogin_RememberMe_BadCredentialsCreateNoSession(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	f.mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", bcryptHash(t, "correct-password"), nil, nil))

	rec := postJSON(f.h.Login, "/api/auth/login", map[string]any{"email": "user@example.com", "password": "wrong", "remember_me": true})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err) // any INSERT would be an unexpected call
	}
}

func TestLogin_RememberMe_ServerWithoutSupportFallsBack(t *testing.T) {
	f := newRememberFixture(t, false)
	defer f.done()
	expectLoginOK(f, t)

	rec := postJSON(f.h.Login, "/api/auth/login", map[string]any{"email": "user@example.com", "password": "correct-password", "remember_me": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := decode(t, rec)["remember_token"]; ok {
		t.Fatal("a server without remembered sessions must not return a remember_token")
	}
}

func TestLogin_RememberMe_StorageFailureIsNotAnnouncedAsSuccess(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	expectLoginOK(f, t)
	f.mock.ExpectBegin()
	f.mock.ExpectExec(`INSERT INTO remember_sessions`).WillReturnError(driver.ErrBadConn)
	f.mock.ExpectRollback()

	rec := postJSON(f.h.Login, "/api/auth/login", map[string]any{"email": "user@example.com", "password": "correct-password", "remember_me": true})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500 rather than a session that will not survive", rec.Code)
	}
}

// --- 1.3 refresh -------------------------------------------------------

func expectRefreshReads(f *rememberFixture, token string, expires time.Time, revoked *time.Time, forbidden *time.Time) {
	f.mock.ExpectQuery(`SELECT \* FROM remember_sessions WHERE token_hash`).
		WithArgs(remember.Hash(token)).
		WillReturnRows(sessionRow(3, 1, remember.Hash(token), expires, revoked))
	_ = forbidden
}

func TestRefresh_ValidOnDay12KeepsTheEndDate(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	const token = "valid-token"
	expires := rememberTestEpoch.Add(remember.Lifetime)
	*f.clock = rememberTestEpoch.Add(12 * 24 * time.Hour)

	expectRefreshReads(f, token, expires, nil, nil)
	f.mock.ExpectQuery(`SELECT \* FROM users WHERE id = \$1`).WithArgs(1).WillReturnRows(rememberUserRow(1, nil))
	f.mock.ExpectExec(`^UPDATE remember_sessions SET last_used_at = \$2 WHERE id = \$1$`).
		WithArgs(3, *f.clock).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": token})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if s, _ := body["token"].(string); s == "" {
		t.Fatalf("expected a fresh JWT, got %v", body)
	}
	if _, ok := body["user"]; !ok {
		t.Fatal("the response must carry the user")
	}
	if v, ok := body["password_change_required"]; !ok || v != false {
		t.Fatalf("password_change_required = %v", v)
	}
	if _, ok := body["remember_expires_at"]; ok {
		t.Fatal("a refresh must not hand out a new end date")
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err) // the only write is last_used_at: expires_at is never rewritten
	}
}

func TestRefresh_ExpiredUnknownAndRevokedAnswerIdentically(t *testing.T) {
	expires := rememberTestEpoch.Add(remember.Lifetime)
	revoked := rememberTestEpoch.Add(time.Hour)
	cases := map[string]func(f *rememberFixture){
		"expired (day 30 and one minute)": func(f *rememberFixture) {
			*f.clock = expires.Add(time.Minute)
			expectRefreshReads(f, "tok", expires, nil, nil)
		},
		"unknown": func(f *rememberFixture) {
			f.mock.ExpectQuery(`SELECT \* FROM remember_sessions WHERE token_hash`).
				WithArgs(remember.Hash("tok")).WillReturnRows(sqlmock.NewRows(rememberRowColumns))
		},
		"revoked": func(f *rememberFixture) {
			*f.clock = rememberTestEpoch.Add(24 * time.Hour)
			expectRefreshReads(f, "tok", expires, &revoked, nil)
		},
	}
	var bodies []string
	for name, setup := range cases {
		f := newRememberFixture(t, true)
		setup(f)
		rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": "tok"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, rec.Code)
		}
		if decode(t, rec)["error"] != "remember_invalid" {
			t.Errorf("%s: body %s, want remember_invalid", name, rec.Body.String())
		}
		bodies = append(bodies, rec.Body.String())
		if err := f.mock.ExpectationsWereMet(); err != nil {
			t.Errorf("%s: no user read and no write may follow a refusal: %v", name, err)
		}
		f.done()
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Fatalf("the three refusals must be indistinguishable: %q vs %q", bodies[0], b)
		}
	}
}

func TestRefresh_ExactlyAtTheEndIsRefused(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	expires := rememberTestEpoch.Add(remember.Lifetime)
	*f.clock = expires
	expectRefreshReads(f, "tok", expires, nil, nil)
	rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": "tok"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401 at the very end", rec.Code)
	}
}

func TestRefresh_ForbiddenAccountIs403AndNoToken(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	forbidden := rememberTestEpoch.Add(time.Hour)
	expectRefreshReads(f, "tok", rememberTestEpoch.Add(remember.Lifetime), nil, nil)
	f.mock.ExpectQuery(`SELECT \* FROM users WHERE id = \$1`).WithArgs(1).WillReturnRows(rememberUserRow(1, &forbidden))

	rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": "tok"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	body := decode(t, rec)
	if body["error"] != "account_forbidden" {
		t.Fatalf("body %v", body)
	}
	if _, ok := body["token"]; ok {
		t.Fatal("no JWT for a forbidden account")
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err) // no Touch either
	}
}

func TestRefresh_EmptyOrMalformedBody(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	if rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": "  "}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("blank token: %d, want 401", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/remember/refresh", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	f.h.RememberRefresh(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: %d, want 400", rec.Code)
	}
}

func TestRefresh_ServerWithoutSupportIs503(t *testing.T) {
	f := newRememberFixture(t, false)
	defer f.done()
	if rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": "x"}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
}

func TestRefresh_BurstIsRateLimited(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	limited := rememberRateLimitMiddleware(middleware.NewRateLimiter(2, time.Hour))(http.HandlerFunc(f.h.RememberRefresh))
	var codes []int
	for i := 0; i < 3; i++ {
		raw, _ := json.Marshal(map[string]string{"remember_token": ""})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/remember/refresh", bytes.NewReader(raw))
		req.RemoteAddr = "203.0.113.9:4000"
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[0] != http.StatusUnauthorized || codes[1] != http.StatusUnauthorized || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes %v, want [401 401 429]", codes)
	}
}

// --- 1.4 end of session ------------------------------------------------

func TestLogout_WithTokenRevokesThatSession(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	f.mock.ExpectExec(`UPDATE remember_sessions SET revoked_at = \$2\s+WHERE token_hash = \$1 AND revoked_at IS NULL`).
		WithArgs(remember.Hash("tok"), *f.clock).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := postJSON(f.h.Logout, "/api/auth/logout", map[string]string{"remember_token": "tok"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	// The next refresh with that token reads a revoked row and is refused.
	revoked := *f.clock
	expectRefreshReads(f, "tok", rememberTestEpoch.Add(remember.Lifetime), &revoked, nil)
	if rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": "tok"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: %d, want 401", rec.Code)
	}
}

func TestLogout_WithoutBodyOrWithGarbageIs200AndTouchesNothing(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	for name, body := range map[string]string{"no body": "", "garbage": "{nope", "empty token": `{"remember_token":""}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", strings.NewReader(body))
		rec := httptest.NewRecorder()
		f.h.Logout(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, want 200", name, rec.Code)
		}
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err) // other sessions of the user stay valid: no write at all
	}
}

func TestLogout_StorageFailureStillAnswers200(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	f.mock.ExpectExec(`UPDATE remember_sessions SET revoked_at`).WillReturnError(driver.ErrBadConn)
	if rec := postJSON(f.h.Logout, "/api/auth/logout", map[string]string{"remember_token": "tok"}); rec.Code != http.StatusOK {
		t.Fatalf("status %d, an app signing out must never be told it failed", rec.Code)
	}
}

func TestLogout_ServerWithoutSupportStaysAPlain200(t *testing.T) {
	f := newRememberFixture(t, false)
	defer f.done()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	f.h.Logout(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

// Two devices, then a password change: one statement ends every session of the
// account (by user id, not by token), in the same transaction as the password.
func TestChangePassword_RevokesEveryRememberedSessionThenBothRefreshesFail(t *testing.T) {
	f := newRememberFixture(t, true)
	defer f.done()
	f.mock.ExpectQuery(`SELECT \* FROM users WHERE email`).WithArgs("user@example.com").
		WillReturnRows(loginUserRow("user@example.com", bcryptHash(t, "current-password"), nil, nil))
	f.mock.ExpectBegin()
	f.mock.ExpectExec(`UPDATE users`).WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectExec(`UPDATE remember_sessions SET revoked_at = NOW\(\)\s+WHERE user_id = \$1 AND revoked_at IS NULL`).
		WithArgs(1).WillReturnResult(sqlmock.NewResult(0, 2))
	f.mock.ExpectCommit()

	if rec := callChangePassword(f.h, "user@example.com", "current-password", "a-brand-new-password"); rec.Code != http.StatusOK {
		t.Fatalf("change password: %d %s", rec.Code, rec.Body.String())
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	revoked := *f.clock
	for _, tok := range []string{"phone", "tablet"} {
		expectRefreshReads(f, tok, rememberTestEpoch.Add(remember.Lifetime), &revoked, nil)
		if rec := postJSON(f.h.RememberRefresh, "/api/auth/remember/refresh", map[string]string{"remember_token": tok}); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: refresh after password change = %d, want 401", tok, rec.Code)
		}
	}
}
