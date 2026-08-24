package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/mailtpl"
	"github.com/jmoiron/sqlx"
)

// stubTemplates stands in for the email_templates table.
type stubTemplates struct{}

func (stubTemplates) Get(slug string) (*domain.EmailTemplate, error) {
	return &domain.EmailTemplate{Slug: slug, Enabled: true, Subject: "s", BodyHTML: "b"}, nil
}

func (stubTemplates) LogSend(recipient, slug, status, errMsg string, adminID *int) error {
	return nil
}

// newForgotHandlers captures dispatched work rather than running it in a
// goroutine that would outlive the test, and counts it to prove the eligible
// path is the only one that mails. The mailer is non-nil because dispatchMail
// rightly declines to schedule anything when there is nowhere to send.
func newForgotHandlers(t *testing.T, enforceTurnstile bool, verifier *stubVerifier) (*Handlers, sqlmock.Sqlmock, *int, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	dispatched := 0
	h := NewHandlers(
		data.NewUserStore(sqlxDB),
		verifier,
		enforceTurnstile,
		WithPasswordResets(data.NewPasswordResetStore(sqlxDB)),
		WithMailer(mailtpl.NewSender(stubTemplates{})),
	)
	h.dispatch = func(func()) { dispatched++ }
	return h, mock, &dispatched, func() { _ = db.Close() }
}

func postForgot(t *testing.T, h *Handlers, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ForgotPassword(rec, httptest.NewRequest(http.MethodPost, "/api/auth/password/forgot", strings.NewReader(body)))
	return rec
}

func expectUserByEmail(mock sqlmock.Sqlmock, email string, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).WithArgs(email).WillReturnRows(rows)
}

// expectIssue mirrors PasswordResetStore.Issue: throttle count, invalidate the
// previous tokens, insert the new one, then purge. The store writes count(*) in
// lower case and sqlmock matches the query text case-sensitively.
func expectIssue(mock sqlmock.Sqlmock, userID, recent int) {
	mock.ExpectQuery(`SELECT count\(\*\) FROM password_reset_tokens`).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(recent))
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO password_reset_tokens`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`DELETE FROM password_reset_tokens`).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestForgotAcceptsKnownAccountAndSchedulesMail(t *testing.T) {
	h, mock, dispatched, done := newForgotHandlers(t, false, &stubVerifier{})
	defer done()

	expectUserByEmail(mock, "emilien@example.com", userRows(12, "emilien@example.com", "hash", nil))
	expectIssue(mock, 12, 0)

	rec := postForgot(t, h, `{"email":"emilien@example.com"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	if *dispatched != 1 {
		t.Fatalf("expected mail scheduled once, got %d", *dispatched)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestForgotUnknownAddressIsIndistinguishable(t *testing.T) {
	known, mockK, dispatchedK, doneK := newForgotHandlers(t, false, &stubVerifier{})
	defer doneK()
	expectUserByEmail(mockK, "known@example.com", userRows(12, "known@example.com", "hash", nil))
	expectIssue(mockK, 12, 0)
	hit := postForgot(t, known, `{"email":"known@example.com"}`)

	unknown, mockU, dispatchedU, doneU := newForgotHandlers(t, false, &stubVerifier{})
	defer doneU()
	expectUserByEmail(mockU, "nobody@example.com", sqlmock.NewRows([]string{"id"}))
	miss := postForgot(t, unknown, `{"email":"nobody@example.com"}`)

	if hit.Code != miss.Code {
		t.Fatalf("status differs: hit=%d miss=%d", hit.Code, miss.Code)
	}
	if hit.Body.String() != miss.Body.String() {
		t.Fatalf("body differs:\n hit=%q\nmiss=%q", hit.Body.String(), miss.Body.String())
	}
	if hit.Header().Get("Content-Type") != miss.Header().Get("Content-Type") {
		t.Fatal("content type differs between known and unknown address")
	}
	if *dispatchedK != 1 {
		t.Fatalf("known address should schedule mail, got %d", *dispatchedK)
	}
	// The decisive assertion: the uniform body must not come with a token.
	if *dispatchedU != 0 {
		t.Fatalf("unknown address must not schedule mail, got %d", *dispatchedU)
	}
}

func TestForgotDoesNotReopenForbiddenAccount(t *testing.T) {
	h, mock, dispatched, done := newForgotHandlers(t, false, &stubVerifier{})
	defer done()

	forbidden := time.Now().Add(-time.Hour)
	expectUserByEmail(mock, "banned@example.com", userRows(12, "banned@example.com", "hash", &forbidden))

	rec := postForgot(t, h, `{"email":"banned@example.com"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected the uniform 202, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Si un compte existe") {
		t.Fatalf("forbidden account leaked its state: %s", rec.Body.String())
	}
	if *dispatched != 0 {
		t.Fatal("a forbidden account must not receive a reset link")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestForgotThrottlesPerAccount(t *testing.T) {
	h, mock, dispatched, done := newForgotHandlers(t, false, &stubVerifier{})
	defer done()

	expectUserByEmail(mock, "spammed@example.com", userRows(12, "spammed@example.com", "hash", nil))
	mock.ExpectQuery(`SELECT count\(\*\) FROM password_reset_tokens`).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(maxTokensPerAccountPerHour))

	rec := postForgot(t, h, `{"email":"spammed@example.com"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("throttling must stay invisible, got %d", rec.Code)
	}
	if *dispatched != 0 {
		t.Fatal("no further mail once the per-account cap is reached")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestForgotRejectsMalformedEmail(t *testing.T) {
	for _, body := range []string{
		`{"email":""}`,
		`{"email":"   "}`,
		`{"email":"not-an-email"}`,
		`{}`,
	} {
		h, _, dispatched, done := newForgotHandlers(t, false, &stubVerifier{})
		rec := postForgot(t, h, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", body, rec.Code)
		}
		var payload map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &payload)
		if payload["error"] != "invalid_email" {
			t.Fatalf("%s: expected invalid_email, got %q", body, payload["error"])
		}
		if *dispatched != 0 {
			t.Fatalf("%s: nothing should be scheduled", body)
		}
		done()
	}
}

func TestForgotRequiresCaptchaBeforeTouchingTheDatabase(t *testing.T) {
	// No sqlmock expectations are registered: if the handler reached the user
	// lookup it would fail, which is the point.
	h, _, dispatched, done := newForgotHandlers(t, true, &stubVerifier{})
	defer done()

	rec := postForgot(t, h, `{"email":"emilien@example.com"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var payload map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["error"] != "captcha_required" {
		t.Fatalf("expected captcha_required, got %q", payload["error"])
	}
	if *dispatched != 0 {
		t.Fatal("nothing should be scheduled without a captcha")
	}
}

func TestForgotRejectsFailedCaptcha(t *testing.T) {
	h, _, dispatched, done := newForgotHandlers(t, true, &stubVerifier{err: context.Canceled})
	defer done()

	rec := postForgot(t, h, `{"email":"emilien@example.com","turnstile_token":"tok"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	var payload map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["error"] != "captcha_failed" {
		t.Fatalf("expected captcha_failed, got %q", payload["error"])
	}
	if *dispatched != 0 {
		t.Fatal("a rejected captcha must not schedule mail")
	}
}

func TestResetVarsCarryTheLinkAndSuppressTheOldMarker(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://www.essensys.fr/")
	h := NewHandlers(nil, &stubVerifier{}, false)
	user := &domain.User{ID: 12, Email: "emilien@example.com", FirstName: "Emilien"}

	vars := h.resetVars(user, "plain-token", time.Now().Add(45*time.Minute))

	if got := vars["reset_url"]; got != "https://www.essensys.fr/reset-password?token=plain-token" {
		t.Fatalf("reset_url = %q", got)
	}
	// Left non-empty, notify.Render would print the placeholder verbatim in a
	// template still carrying the pre-013 wording.
	if vars["temporary_password"] != "" {
		t.Fatalf("temporary_password should be pinned empty, got %q", vars["temporary_password"])
	}
	if vars["expires_in"] != "44" && vars["expires_in"] != "45" {
		t.Fatalf("expires_in = %q, want the remaining minutes", vars["expires_in"])
	}
	if vars["email"] != user.Email || vars["first_name"] != "Emilien" {
		t.Fatalf("base user vars missing: %#v", vars)
	}
}

func TestForgotWithoutResetStoreIsUnavailable(t *testing.T) {
	h := NewHandlers(nil, &stubVerifier{}, false)
	if code := postForgot(t, h, `{"email":"a@b.c"}`).Code; code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", code)
	}
}
