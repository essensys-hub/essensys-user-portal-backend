package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/jmoiron/sqlx"
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
