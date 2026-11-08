package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/jmoiron/sqlx"
)

// sonnette-plages-push-2026-11-008 (#61): gateway_state in GET /api/portal/session.

const sessionPath = "/api/portal/session"

func newSessionEnv(t *testing.T, withInventory bool) *ltEnv {
	t.Helper()
	t.Setenv("NEW_RELIC_ENABLED", "false")
	t.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	sx := sqlx.NewDb(db, "sqlmock")
	var inv *data.AdminInventoryStore
	if withInventory {
		inv = data.NewAdminInventoryStore(sx)
	}
	cfg := config.Config{ConsolidatedMode: true, ExchangeStaleTTL: 120 * time.Second, CORSOrigin: "https://mon.essensys.fr"}
	return &ltEnv{handler: NewRouter(data.NewPortalStore(sx), data.NewUserStore(sx), nil, inv, nil, nil, nil, nil, nil, cfg), mock: mock}
}

type sessionOut struct {
	GatewayState string         `json:"gateway_state"`
	Gateway      map[string]any `json:"gateway"`
}

func getSession(t *testing.T, e *ltEnv) (sessionOut, string) {
	t.Helper()
	rec := e.do(t, http.MethodGet, sessionPath, "a@example.org", "user", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out sessionOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out, rec.Body.String()
}

func lastSeenRow(ago time.Duration) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"last_seen"}).AddRow(time.Now().Add(-ago))
}

func TestPortalSession_gatewayStateNone(t *testing.T) {
	e := newSessionEnv(t, false)
	e.pushUser(7, "a@example.org", nil)
	out, raw := getSession(t, e)
	if out.GatewayState != "none" || out.Gateway != nil {
		t.Fatalf("state %q gateway %v: %s", out.GatewayState, out.Gateway, raw)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// essensys-server (the legacy host) is not eligible for the remote portal.
func TestPortalSession_gatewayNonEligibleEstNone(t *testing.T) {
	e := newSessionEnv(t, false)
	e.pushUser(7, "a@example.org", "essensys-server")
	out, raw := getSession(t, e)
	if out.GatewayState != "none" {
		t.Fatalf("state %q: %s", out.GatewayState, raw)
	}
}

func TestPortalSession_gatewayStateOnlineA30s(t *testing.T) {
	e := newSessionEnv(t, false)
	e.pushUser(7, "a@example.org", "gw-1")
	e.mock.ExpectQuery(`SELECT last_seen FROM gateway_sessions`).WithArgs("gw-1").WillReturnRows(lastSeenRow(30 * time.Second))
	if out, raw := getSession(t, e); out.GatewayState != "online" {
		t.Fatalf("state %q: %s", out.GatewayState, raw)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPortalSession_gatewayStateOfflineA3min(t *testing.T) {
	e := newSessionEnv(t, false)
	e.pushUser(7, "a@example.org", "gw-1")
	e.mock.ExpectQuery(`SELECT last_seen FROM gateway_sessions`).WithArgs("gw-1").WillReturnRows(lastSeenRow(3 * time.Minute))
	if out, raw := getSession(t, e); out.GatewayState != "offline" {
		t.Fatalf("state %q: %s", out.GatewayState, raw)
	}
}

func TestPortalSession_gatewayJamaisVueEstOffline(t *testing.T) {
	e := newSessionEnv(t, false)
	e.pushUser(7, "a@example.org", "gw-1")
	e.mock.ExpectQuery(`SELECT last_seen FROM gateway_sessions`).WithArgs("gw-1").WillReturnRows(sqlmock.NewRows([]string{"last_seen"}))
	if out, raw := getSession(t, e); out.GatewayState != "offline" {
		t.Fatalf("state %q: %s", out.GatewayState, raw)
	}
}

// The existing gateway block stays (older clients rely on it).
func TestPortalSession_blocGatewayConserve(t *testing.T) {
	e := newSessionEnv(t, true)
	e.pushUser(7, "a@example.org", "gw-1")
	e.mock.ExpectQuery(`SELECT last_seen FROM gateway_sessions`).WithArgs("gw-1").WillReturnRows(lastSeenRow(30 * time.Second))
	e.mock.ExpectQuery(`FROM gateway_push_status`).WillReturnRows(sqlmock.NewRows([]string{"hostname", "payload", "updated_at"}))
	e.mock.ExpectQuery(`SELECT last_seen FROM gateway_sessions`).WithArgs("gw-1").WillReturnRows(lastSeenRow(30 * time.Second))
	out, raw := getSession(t, e)
	if out.GatewayState != "online" || out.Gateway == nil || out.Gateway["id"] != "gw-1" || out.Gateway["online"] != true {
		t.Fatalf("state %q gateway %v: %s", out.GatewayState, out.Gateway, raw)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
