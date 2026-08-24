package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/pwreset"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const testToken = "test-token-value"

func newResetHandlers(t *testing.T) (*Handlers, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	h := NewHandlers(
		data.NewUserStore(sqlxDB),
		&stubVerifier{},
		false,
		WithPasswordResets(data.NewPasswordResetStore(sqlxDB)),
	)
	return h, mock, func() { _ = db.Close() }
}

// tokenRows builds what GetByHash sees. consumedAt/invalidatedAt nil means live.
func tokenRows(userID int, expiresAt time.Time, consumedAt, invalidatedAt *time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "user_id", "token_hash", "expires_at", "consumed_at", "invalidated_at", "request_ip", "consumed_ip", "created_at"}).
		AddRow(1, userID, pwreset.Hash(testToken), expiresAt, consumedAt, invalidatedAt, nil, nil, time.Now())
}

func userRows(id int, email, passwordHash string, forbiddenAt *time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "email", "password_hash", "role", "first_name", "last_name", "provider", "provider_id", "created_at", "last_login", "forbidden_at"}).
		AddRow(id, email, passwordHash, "user", "Emilien", "B", "email", "", time.Now(), time.Now(), forbiddenAt)
}

func expectToken(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM password_reset_tokens WHERE token_hash`).
		WithArgs(pwreset.Hash(testToken)).WillReturnRows(rows)
}

func expectUser(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).WithArgs(12).WillReturnRows(rows)
}

func postReset(t *testing.T, h *Handlers, token, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"token": token, "password": password})
	rec := httptest.NewRecorder()
	h.ResetPassword(rec, httptest.NewRequest(http.MethodPost, "/api/auth/password/reset", strings.NewReader(string(body))))
	return rec
}

func TestResetEndpointsWithoutStore(t *testing.T) {
	h := NewHandlers(nil, &stubVerifier{}, false)

	rec := httptest.NewRecorder()
	h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/api/auth/password/reset/validate?token=x", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("validate: expected 503, got %d", rec.Code)
	}

	if code := postReset(t, h, "x", "longenough").Code; code != http.StatusServiceUnavailable {
		t.Fatalf("reset: expected 503, got %d", code)
	}
}

func TestValidateResetToken(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	now := time.Now()
	future := time.Now().Add(30 * time.Minute)

	t.Run("valid token returns masked email", func(t *testing.T) {
		h, mock, done := newResetHandlers(t)
		defer done()
		expectToken(mock, tokenRows(12, future, nil, nil))
		expectUser(mock, userRows(12, "emilienbieber67260@gmail.com", "hash", nil))

		rec := httptest.NewRecorder()
		h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/x?token="+testToken, nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["valid"] != true {
			t.Fatalf("expected valid, got %s", rec.Body.String())
		}
		if body["email_masked"] != "e***@gmail.com" {
			t.Fatalf("expected masked email, got %v", body["email_masked"])
		}
		if strings.Contains(rec.Body.String(), "emilienbieber67260") {
			t.Fatal("full address must not be disclosed")
		}
	})

	t.Run("expired token is 410 expired", func(t *testing.T) {
		h, mock, done := newResetHandlers(t)
		defer done()
		expectToken(mock, tokenRows(12, past, nil, nil))

		rec := httptest.NewRecorder()
		h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/x?token="+testToken, nil))
		if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), "expired") {
			t.Fatalf("expected 410 expired, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("consumed token is 410 used", func(t *testing.T) {
		h, mock, done := newResetHandlers(t)
		defer done()
		expectToken(mock, tokenRows(12, future, &now, nil))

		rec := httptest.NewRecorder()
		h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/x?token="+testToken, nil))
		if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), "used") {
			t.Fatalf("expected 410 used, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalidated token is 410 used", func(t *testing.T) {
		h, mock, done := newResetHandlers(t)
		defer done()
		expectToken(mock, tokenRows(12, future, nil, &now))

		rec := httptest.NewRecorder()
		h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/x?token="+testToken, nil))
		if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), "used") {
			t.Fatalf("expected 410 used, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown token is 400 invalid", func(t *testing.T) {
		h, mock, done := newResetHandlers(t)
		defer done()
		expectToken(mock, sqlmock.NewRows([]string{"id"}))

		rec := httptest.NewRecorder()
		h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/x?token="+testToken, nil))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid") {
			t.Fatalf("expected 400 invalid, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing parameter is 400", func(t *testing.T) {
		h, _, done := newResetHandlers(t)
		defer done()

		rec := httptest.NewRecorder()
		h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

// Pre-validation is what mail scanners hit. It must leave the token alone.
func TestValidateResetTokenHasNoSideEffect(t *testing.T) {
	h, mock, done := newResetHandlers(t)
	defer done()
	future := time.Now().Add(30 * time.Minute)

	for i := 0; i < 3; i++ {
		expectToken(mock, tokenRows(12, future, nil, nil))
		expectUser(mock, userRows(12, "a@b.c", "hash", nil))
		rec := httptest.NewRecorder()
		h.ValidateResetToken(rec, httptest.NewRequest(http.MethodGet, "/x?token="+testToken, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: expected 200, got %d", i, rec.Code)
		}
	}
	// No UPDATE or DELETE was expected, so any write would fail the mock here.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResetPasswordSucceeds(t *testing.T) {
	h, mock, done := newResetHandlers(t)
	defer done()
	future := time.Now().Add(30 * time.Minute)

	expectToken(mock, tokenRows(12, future, nil, nil))
	expectUser(mock, userRows(12, "a@b.c", mustHash(t, "oldpassword"), nil))
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(12))
	mock.ExpectExec(`UPDATE users SET password_hash`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	rec := postReset(t, h, testToken, "brandnewpassword")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A weak or reused password is the user's mistake, not an attack. Burning the
// link would send them back to their mailbox for nothing.
func TestResetPasswordRejectionsDoNotConsumeToken(t *testing.T) {
	future := time.Now().Add(30 * time.Minute)

	t.Run("too short", func(t *testing.T) {
		h, mock, done := newResetHandlers(t)
		defer done()
		expectToken(mock, tokenRows(12, future, nil, nil))
		expectUser(mock, userRows(12, "a@b.c", mustHash(t, "oldpassword"), nil))

		rec := postReset(t, h, testToken, "short")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "weak_password") {
			t.Fatalf("expected 400 weak_password, got %d %s", rec.Code, rec.Body.String())
		}
		// No Begin was expected: reaching the transaction would fail the mock.
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("same as current", func(t *testing.T) {
		h, mock, done := newResetHandlers(t)
		defer done()
		expectToken(mock, tokenRows(12, future, nil, nil))
		expectUser(mock, userRows(12, "a@b.c", mustHash(t, "oldpassword"), nil))

		rec := postReset(t, h, testToken, "oldpassword")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "password_reused") {
			t.Fatalf("expected 400 password_reused, got %d %s", rec.Code, rec.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestResetPasswordOnForbiddenAccount(t *testing.T) {
	h, mock, done := newResetHandlers(t)
	defer done()
	future := time.Now().Add(30 * time.Minute)
	forbidden := time.Now()

	expectToken(mock, tokenRows(12, future, nil, nil))
	expectUser(mock, userRows(12, "a@b.c", mustHash(t, "oldpassword"), &forbidden))
	// The link must be retired so a later un-forbid does not reopen it.
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).
		WithArgs(12).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := postReset(t, h, testToken, "brandnewpassword")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "account_forbidden") {
		t.Fatalf("expected 403 account_forbidden, got %d %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResetPasswordSecondUseIsRejected(t *testing.T) {
	h, mock, done := newResetHandlers(t)
	defer done()
	future := time.Now().Add(30 * time.Minute)
	consumed := time.Now()

	// Replaying a spent link: rejected before any password work happens.
	expectToken(mock, tokenRows(12, future, &consumed, nil))

	rec := postReset(t, h, testToken, "brandnewpassword")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_token") {
		t.Fatalf("expected 400 invalid_token, got %d %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// If a concurrent request consumed the token between our read and our write,
// the loser must get the same opaque answer as any invalid token.
func TestResetPasswordLosesConsumeRace(t *testing.T) {
	h, mock, done := newResetHandlers(t)
	defer done()
	future := time.Now().Add(30 * time.Minute)

	expectToken(mock, tokenRows(12, future, nil, nil))
	expectUser(mock, userRows(12, "a@b.c", mustHash(t, "oldpassword"), nil))
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectRollback()

	rec := postReset(t, h, testToken, "brandnewpassword")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_token") {
		t.Fatalf("expected 400 invalid_token, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestResetPasswordRejectsEmptyOrMalformedInput(t *testing.T) {
	h, _, done := newResetHandlers(t)
	defer done()

	if code := postReset(t, h, "   ", "longenough").Code; code != http.StatusBadRequest {
		t.Fatalf("blank token: expected 400, got %d", code)
	}

	rec := httptest.NewRecorder()
	h.ResetPassword(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("not json")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: expected 400, got %d", rec.Code)
	}
}

func TestResetPasswordResponseLeaksNothing(t *testing.T) {
	h, mock, done := newResetHandlers(t)
	defer done()
	future := time.Now().Add(30 * time.Minute)
	consumed := time.Now()
	expectToken(mock, tokenRows(12, future, &consumed, nil))

	body := strings.ToLower(postReset(t, h, testToken, "brandnewpassword").Body.String())
	for _, leak := range []string{"@", "hash", "digest", "sha", "bcrypt", "expired", "consumed", "sql"} {
		if strings.Contains(body, leak) {
			t.Fatalf("response leaked %q: %s", leak, body)
		}
	}
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}
