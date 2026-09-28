package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/temppass"
	"github.com/go-chi/chi/v5"
)

func callIssue(h *Handlers, id, callerEmail string, body map[string]any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+id+"/temporary-password", reader)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if callerEmail != "" {
		ctx = context.WithValue(ctx, middleware.UserEmailKey, callerEmail)
	}
	rec := httptest.NewRecorder()
	h.IssueTemporaryPassword(rec, req.WithContext(ctx))
	return rec
}

func expectSetTemporaryPassword(mock sqlmock.Sqlmock, userID int) {
	mock.ExpectExec(`(?s)UPDATE users\s+SET password_hash = \$1,\s+password_change_required_at = NOW\(\),\s+temp_password_expires_at = \$2,\s+temp_password_issued_by = \$3\s+WHERE id = \$4`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectInvalidateResetTokens(mock sqlmock.Sqlmock, userID int) {
	mock.ExpectExec(`UPDATE password_reset_tokens SET invalidated_at = NOW\(\)`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestIssueTemporaryPasswordRequiresGlobalAdmin(t *testing.T) {
	t.Run("no caller in context is 401", func(t *testing.T) {
		h, mock, done := newResetAdmin(t, true)
		defer done()
		mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		if code := callIssue(h, "12", "", nil).Code; code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", code)
		}
	})

	t.Run("non-admin caller is 403", func(t *testing.T) {
		h, mock, done := newResetAdmin(t, true)
		defer done()
		mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
			WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleUser))

		if code := callIssue(h, "12", adminEmail, nil).Code; code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", code)
		}
	})
}

func TestIssueTemporaryPasswordUnknownUserIs404(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(999).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if code := callIssue(h, "999", adminEmail, nil).Code; code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestIssueTemporaryPasswordInvalidIDIs400(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))

	if code := callIssue(h, "abc", adminEmail, nil).Code; code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

// Same rule as the reset-link endpoint: issuing a fresh credential to a
// forbidden account would quietly reopen it.
func TestIssueTemporaryPasswordOnForbiddenAccountIs409(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	forbidden := time.Now()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(12).WillReturnRows(targetUserRows(&forbidden))

	rec := callIssue(h, "12", adminEmail, nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "account_forbidden") {
		t.Fatalf("expected 409 account_forbidden, got %d %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueTemporaryPasswordSucceeds(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(12).WillReturnRows(targetUserRows(nil))
	expectSetTemporaryPassword(mock, 12)
	expectInvalidateResetTokens(mock, 12)

	rec := callIssue(h, "12", adminEmail, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	pwd, _ := body["password"].(string)
	if len(pwd) != temppass.Length {
		t.Fatalf("expected a %d-character password in the response, got %q", temppass.Length, pwd)
	}
	if body["email_sent"] != false {
		t.Fatalf("expected email_sent=false when send_email was not requested, got %v", body["email_sent"])
	}
	if _, ok := body["expires_at"]; !ok {
		t.Fatalf("expected expires_at, got %s", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// h.templates stays nil in this fixture, so sendTemplateEmailWithVars fails
// its precondition check before touching the database — exactly D7: the
// emission already succeeded (SetTemporaryPassword ran), only delivery
// failed, so the response is still 200.
func TestIssueTemporaryPasswordEmailFailureDoesNotUndoIssuance(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(12).WillReturnRows(targetUserRows(nil))
	expectSetTemporaryPassword(mock, 12)
	expectInvalidateResetTokens(mock, 12)

	rec := callIssue(h, "12", adminEmail, map[string]any{"send_email": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	if body["email_sent"] != false {
		t.Fatalf("expected email_sent=false on delivery failure, got %v", body["email_sent"])
	}
	if _, ok := body["reason"]; !ok {
		t.Fatalf("expected a failure reason, got %s", rec.Body.String())
	}
	if _, ok := body["password"]; !ok {
		t.Fatal("password must still be returned even when delivery failed")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueTemporaryPasswordWithoutSendEmailSkipsDelivery(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(12).WillReturnRows(targetUserRows(nil))
	expectSetTemporaryPassword(mock, 12)
	expectInvalidateResetTokens(mock, 12)

	// No body at all: the default must be "don't send", not a bad request.
	rec := callIssue(h, "12", adminEmail, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if _, hasReason := body["reason"]; hasReason {
		t.Fatalf("no delivery was requested, so no failure reason should appear: %s", rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The generated plaintext must never reach the application log — neither on
// the success path nor when the optional email fails to send.
func TestIssueTemporaryPasswordNeverLogsThePlaintext(t *testing.T) {
	var logBuf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(orig)

	h, mock, done := newResetAdmin(t, true)
	defer done()
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
		WithArgs(12).WillReturnRows(targetUserRows(nil))
	expectSetTemporaryPassword(mock, 12)
	expectInvalidateResetTokens(mock, 12)

	rec := callIssue(h, "12", adminEmail, map[string]any{"send_email": true})

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	pwd, _ := body["password"].(string)
	if pwd == "" {
		t.Fatal("expected a password in the response to check against the log")
	}

	if strings.Contains(logBuf.String(), pwd) {
		t.Fatalf("application log contains the plaintext password: %s", logBuf.String())
	}
}

// A second issuance is an independent, unconditional UPDATE: it overwrites
// password_hash, the expiry and the issuing admin for the same user rather
// than appending or requiring the first temporary password to be consumed
// first. That a login with the superseded password then fails is exercised
// end-to-end in internal/identity, once ChangePassword/Login exist (section
// 5) — this test covers the backend-observable half of task 4.6.
func TestIssueTemporaryPasswordTwiceOverwritesTheFirst(t *testing.T) {
	h, mock, done := newResetAdmin(t, true)
	defer done()

	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
			WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
		mock.ExpectQuery(`SELECT \* FROM users WHERE id`).
			WithArgs(12).WillReturnRows(targetUserRows(nil))
		expectSetTemporaryPassword(mock, 12)
		expectInvalidateResetTokens(mock, 12)
	}

	first := callIssue(h, "12", adminEmail, nil)
	second := callIssue(h, "12", adminEmail, nil)

	var firstBody, secondBody map[string]any
	_ = json.Unmarshal(first.Body.Bytes(), &firstBody)
	_ = json.Unmarshal(second.Body.Bytes(), &secondBody)

	if firstBody["password"] == secondBody["password"] {
		t.Fatal("two issuances produced the same password — generation is not being exercised")
	}
	// Both UPDATEs were expected unconditionally (no read-before-write guard),
	// which is what makes the second call supersede the first at the database
	// level: mock.ExpectationsWereMet below fails otherwise.

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestGetUsers_ExposesTemporaryPasswordState covers task 4.4: the admin
// user list must surface enough state for the UI badge (task 6.3) without
// exposing the issuing admin's identity, which stays internal
// (temp_password_issued_by has json:"-").
func TestGetUsers_ExposesTemporaryPasswordState(t *testing.T) {
	h, mock, done := newResetAdmin(t, false)
	defer done()

	changeRequiredAt := time.Now().Add(-time.Hour)
	expiresAt := time.Now().Add(71 * time.Hour)

	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs(adminEmail).WillReturnRows(adminUserRows(domain.RoleAdminGlobal))
	mock.ExpectQuery(`SELECT id, email, role, first_name, last_name, provider, created_at, last_login, forbidden_at`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "role", "first_name", "last_name", "provider", "created_at", "last_login", "forbidden_at",
			"linked_machine_id", "linked_gateway_id", "linked_armoire_id",
			"password_change_required_at", "temp_password_expires_at",
		}).
			AddRow(12, "marked@example.com", domain.RoleUser, "M", "", "email", time.Now(), time.Now(), nil,
				nil, nil, nil, changeRequiredAt, expiresAt).
			AddRow(13, "ordinary@example.com", domain.RoleUser, "O", "", "email", time.Now(), time.Now(), nil,
				nil, nil, nil, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	ctx := context.WithValue(req.Context(), middleware.UserEmailKey, adminEmail)
	rec := httptest.NewRecorder()
	h.GetUsers(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var users []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("bad json: %s", rec.Body.String())
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	marked, ordinary := users[0], users[1]
	if _, ok := marked["password_change_required_at"]; !ok {
		t.Fatalf("marked account missing password_change_required_at: %v", marked)
	}
	if _, ok := marked["temp_password_expires_at"]; !ok {
		t.Fatalf("marked account missing temp_password_expires_at: %v", marked)
	}
	if _, ok := marked["temp_password_issued_by"]; ok {
		t.Fatalf("temp_password_issued_by must never be exposed: %v", marked)
	}
	if _, ok := ordinary["password_change_required_at"]; ok {
		t.Fatalf("ordinary account should omit password_change_required_at entirely: %v", ordinary)
	}
	if _, ok := ordinary["temp_password_expires_at"]; ok {
		t.Fatalf("ordinary account should omit temp_password_expires_at entirely: %v", ordinary)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
