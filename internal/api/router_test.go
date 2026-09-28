package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// userColumns mirrors domain.User's db tags. SELECT * scans by column name
// via sqlx.StructScan, so every returned row must carry exactly these
// columns for the scan to succeed.
var userColumns = []string{
	"id", "email", "password_hash", "role", "first_name", "last_name",
	"provider", "provider_id", "created_at", "last_login", "forbidden_at",
	"linked_machine_id", "linked_gateway_id", "linked_armoire_id",
	"password_change_required_at", "temp_password_expires_at", "temp_password_issued_by",
}

// pendingChangeAdminRow is one row for an admin_global account that still
// carries an admin-issued temporary password. Used across every route
// family in TestPasswordChangeRequired_BlocksEveryRouteFamily: identity,
// portal and admin all reload the account fresh on every request, so the
// lock applies identically regardless of role.
func pendingChangeAdminRow(email string, changeRequiredAt time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(userColumns).AddRow(
		1, email, "$2a$10$hash", "admin_global", "Admin", "Support",
		"email", "", time.Now(), time.Now(), nil,
		nil, nil, nil,
		changeRequiredAt, nil, nil,
	)
}

func TestHealthWithoutNewRelic(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")

	cfg := config.Config{ExchangeStaleTTL: 120 * time.Second, CORSOrigin: "https://mon.essensys.fr"}
	handler := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/portal/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestConsolidatedModeScaffold(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")

	cfg := config.Config{
		ConsolidatedMode: true,
		ExchangeStaleTTL: 120 * time.Second,
		CORSOrigin:       "https://mon.essensys.fr",
	}
	handler := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/portal/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("consolidated scaffold: expected 200, got %d", rec.Code)
	}
}

func TestGatewayExchangeRouteRegistered(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")

	cfg := config.Config{ExchangeStaleTTL: 120 * time.Second, CORSOrigin: "https://mon.essensys.fr"}
	handler := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/exchange", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	// Without auth middleware credentials, GatewayAuth returns 401 — route exists.
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without gateway auth, got %d", rec.Code)
	}
}

func TestConsolidatedModeAdminLoginRoute(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")
	t.Setenv("ADMIN_TOKEN", "secret-token")

	cfg := config.Config{
		ConsolidatedMode: true,
		ExchangeStaleTTL: 120 * time.Second,
		CORSOrigin:       "https://mon.essensys.fr",
	}
	handler := NewRouter(nil, &data.UserStore{}, nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader([]byte(`{"token":"bad"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("consolidated admin login: expected 401, got %d", rec.Code)
	}
}

// TestPasswordChangeRequired_BlocksEveryRouteFamily is the cross-package
// integration check for spec password-change-enforcement: identity, portal
// and admin each wire their own middleware chain onto the same
// enforceActiveUser, and this proves the lock actually reaches all three
// through the real router — not just through the unit tests of
// internal/middleware in isolation. The account used carries admin_global,
// specifically to confirm the role grants no exemption.
func TestPasswordChangeRequired_BlocksEveryRouteFamily(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")
	t.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	users := data.NewUserStore(sqlx.NewDb(db, "sqlmock"))

	token, err := middleware.GenerateJWT("admin@example.com", "admin_global", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	changeRequiredAt := time.Now().Add(-time.Minute)

	cfg := config.Config{
		ConsolidatedMode: true,
		ExchangeStaleTTL: 120 * time.Second,
		CORSOrigin:       "https://mon.essensys.fr",
	}
	handler := NewRouter(nil, users, nil, nil, nil, nil, nil, nil, nil, cfg)

	routes := []struct {
		name   string
		method string
		path   string
	}{
		{"identity profile", http.MethodGet, "/api/profile"},
		{"portal session", http.MethodGet, "/api/portal/session"},
		{"admin stats", http.MethodGet, "/api/admin/stats"},
	}

	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			mock.ExpectQuery(`SELECT \* FROM users WHERE email = \$1`).
				WithArgs("admin@example.com").
				WillReturnRows(pendingChangeAdminRow("admin@example.com", changeRequiredAt))

			req := httptest.NewRequest(rt.method, rt.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf("%s: status = %d, want %d — body: %s", rt.path, rec.Code, http.StatusConflict, rec.Body.String())
			}
			if got := rec.Body.String(); !bytes.Contains([]byte(got), []byte("password_change_required")) {
				t.Fatalf("%s: body = %q, want it to contain password_change_required", rt.path, got)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestTemporaryPasswordRouteRegistered is the routing-level half of task
// 4.3: without a valid Authorization header the request never reaches
// IssueTemporaryPassword, so a 401 (rather than chi's 404 for an unmatched
// route) is proof the route exists and is wired to AdminAuthWithStore.
func TestTemporaryPasswordRouteRegistered(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")

	cfg := config.Config{
		ConsolidatedMode: true,
		ExchangeStaleTTL: 120 * time.Second,
		CORSOrigin:       "https://mon.essensys.fr",
	}
	handler := NewRouter(nil, &data.UserStore{}, nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/12/temporary-password", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (route exists, auth missing), got %d", rec.Code)
	}
}

func userRowWithLock(email, hash string, changeRequiredAt, tempExpiresAt *time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(userColumns).AddRow(
		1, email, hash, "user", "First", "Last",
		"email", "", time.Now(), time.Now(), nil,
		nil, nil, nil,
		changeRequiredAt, tempExpiresAt, nil,
	)
}

// TestPasswordChangeEndToEnd_SameTokenUnlocksAfterChange is task 5.4: it
// proves the whole point of reloading the account on every request (D2) —
// the JWT issued at login never changes, and yet the exact same token is
// refused before the change and accepted after it, with no reissue in
// between.
func TestPasswordChangeEndToEnd_SameTokenUnlocksAfterChange(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")
	t.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	users := data.NewUserStore(sqlx.NewDb(db, "sqlmock"))

	tempHash, err := bcrypt.GenerateFromPassword([]byte("temp-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	token, err := middleware.GenerateJWT("user@example.com", "user", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	changeRequiredAt := time.Now().Add(-time.Minute)
	tempExpiresAt := time.Now().Add(71 * time.Hour)

	cfg := config.Config{
		ConsolidatedMode: true,
		ExchangeStaleTTL: 120 * time.Second,
		CORSOrigin:       "https://mon.essensys.fr",
	}
	handler := NewRouter(nil, users, nil, nil, nil, nil, nil, nil, nil, cfg)

	// 1. Before the change: the lock blocks /api/profile outright, so the
	// handler's own (redundant) GetUserByEmail is never reached.
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(userRowWithLock("user@example.com", string(tempHash), &changeRequiredAt, &tempExpiresAt))

	req1 := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusConflict {
		t.Fatalf("before change: status = %d, want %d — body: %s", rec1.Code, http.StatusConflict, rec1.Body.String())
	}

	// 2. The change itself: UserJWTAllowPasswordChange loads the account once
	// (still marked, still not forbidden), then ChangePassword loads it again
	// to check the current password, then clears the lock.
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(userRowWithLock("user@example.com", string(tempHash), &changeRequiredAt, &tempExpiresAt))
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(userRowWithLock("user@example.com", string(tempHash), &changeRequiredAt, &tempExpiresAt))
	mock.ExpectExec(`(?s)UPDATE users\s+SET password_hash = \$1,\s+password_change_required_at = NULL,\s+temp_password_expires_at = NULL,\s+temp_password_issued_by = NULL\s+WHERE id = \$2`).
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	changeBody, _ := json.Marshal(map[string]string{
		"current_password": "temp-password",
		"new_password":     "brand-new-password",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/password/change", bytes.NewReader(changeBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("change request: status = %d, want %d — body: %s", rec2.Code, http.StatusOK, rec2.Body.String())
	}

	// 3. Same token, no reissue: the account now reads back clear, so the
	// exact route that returned 409 in step 1 now succeeds.
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(userRowWithLock("user@example.com", string(tempHash), nil, nil))
	mock.ExpectQuery(`SELECT \* FROM users WHERE email`).
		WithArgs("user@example.com").
		WillReturnRows(userRowWithLock("user@example.com", string(tempHash), nil, nil))

	req3 := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("after change: status = %d, want %d — body: %s", rec3.Code, http.StatusOK, rec3.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
