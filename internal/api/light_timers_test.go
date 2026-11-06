package api

import (
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/jmoiron/sqlx"
)

// lumieres-temporisees-2026-11-003 (#53): light timers routes, through the real
// router, with a mocked database. Nothing here touches a real cabinet.

type ltEnv struct {
	handler http.Handler
	mock    sqlmock.Sqlmock
}

func newLTEnv(t *testing.T) *ltEnv {
	t.Helper()
	t.Setenv("NEW_RELIC_ENABLED", "false")
	t.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	sx := sqlx.NewDb(db, "sqlmock")
	cfg := config.Config{ConsolidatedMode: true, ExchangeStaleTTL: 120 * time.Second, CORSOrigin: "https://mon.essensys.fr"}
	h := NewRouter(data.NewPortalStore(sx), data.NewUserStore(sx), nil, nil, nil, nil, nil, nil, nil, cfg)
	return &ltEnv{handler: h, mock: mock}
}

func (e *ltEnv) userRows(email, role string, machine any) {
	e.mock.ExpectQuery(`SELECT \* FROM users WHERE email = \$1`).WithArgs(email).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
			1, email, "$2a$10$hash", role, "A", "B", "email", "", time.Now(), time.Now(), nil,
			machine, nil, nil, nil, nil, nil))
}

func (e *ltEnv) profileRows(email, role string, machine any, approved bool) {
	e.mock.ExpectQuery(`SELECT id, email, role, first_name, last_name`).WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "role", "first_name", "last_name", "linked_machine_id", "linked_gateway_id", "linked_armoire_id"}).
			AddRow(1, email, role, "A", "B", machine, nil, nil))
	if machine == nil || !strings.HasPrefix(role, "admin") && role != "user" {
		return
	}
	rows := sqlmock.NewRows([]string{"status"})
	if approved {
		rows.AddRow("approved")
	}
	e.mock.ExpectQuery(`SELECT status FROM link_requests`).WillReturnRows(rows)
}

func (e *ltEnv) do(t *testing.T, method, path, email, role, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	tok, err := middleware.GenerateJWT(email, role, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *ltEnv) expectWriteChain(machine int) {
	e.mock.ExpectExec(`INSERT INTO cloud_actions`).WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectQuery(`SELECT machine_id, keys, updated_at FROM gateway_exchange_cache`).WillReturnError(errNoRows)
	e.mock.ExpectExec(`INSERT INTO gateway_exchange_cache`).WithArgs(machine, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
}

var errNoRows = sqlNoRows{}

type sqlNoRows struct{}

func (sqlNoRows) Error() string { return "sql: no rows in result set" }

const ltPath = "/api/portal/admin/lights/timers/"

func TestLightTimersPut_roleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		role    string
		machine any
		want    int
		wantExe bool
	}{
		{"admin_local de l'armoire", "admin_local", 3, http.StatusOK, true},
		{"admin_local d'une autre armoire", "admin_local", 7, http.StatusOK, true},
		{"admin_global avec armoire liee", "admin_global", 3, http.StatusOK, true},
		{"admin_global sans armoire", "admin_global", nil, http.StatusForbidden, false},
		{"admin_local sans armoire", "admin_local", nil, http.StatusForbidden, false},
		{"support", "support", 3, http.StatusForbidden, false},
		{"user", "user", 3, http.StatusForbidden, false},
		{"guest_local", "guest_local", 3, http.StatusForbidden, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newLTEnv(t)
			email := "x@example.com"
			// The user session check always runs once; user and guest_local are
			// then stopped by the admin JWT guard, before the handler loads
			// the profile or touches the queue.
			e.userRows(email, c.role, c.machine)
			if c.role != "user" && c.role != "guest_local" {
				e.userRows(email, c.role, c.machine) // admin sub-group session check
				e.profileRows(email, c.role, c.machine, true)
				if c.wantExe {
					e.expectWriteChain(c.machine.(int))
				}
			}
			rec := e.do(t, http.MethodPut, ltPath+"wc2", email, c.role, `{"minutes":30}`)
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet (an expected write is missing) or unexpected: %v", err)
			}
		})
	}
}

func TestLightTimersPut_enqueuesDecimalString(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("a@example.com", "admin_local", 3)
	e.userRows("a@example.com", "admin_local", 3) // admin sub-group session check
	e.profileRows("a@example.com", "admin_local", 3, true)
	e.mock.ExpectExec(`INSERT INTO cloud_actions`).
		WithArgs(sqlmock.AnyArg(), 1, 3, jsonArg(`[{"k":556,"v":"30"}]`), "pending").
		WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectQuery(`SELECT machine_id, keys, updated_at FROM gateway_exchange_cache`).WillReturnError(errNoRows)
	e.mock.ExpectExec(`INSERT INTO gateway_exchange_cache`).WithArgs(3, jsonArg(`[{"k":556,"v":"30"}]`)).WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.com", "portal_light_timer", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, ltPath+"wc2", "a@example.com", "admin_local", `{"minutes":30}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID      string `json:"id"`
		Minutes int    `json:"minutes"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID != "wc2" || out.Minutes != 30 || out.Status != "queued" {
		t.Fatalf("contract body: %s", rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightTimersPut_validation400SansAction(t *testing.T) {
	cases := []struct{ name, id, body string }{
		{"256", "wc1", `{"minutes":256}`},
		{"-1", "wc1", `{"minutes":-1}`},
		{"7.5", "wc1", `{"minutes":7.5}`},
		{"chaine", "wc1", `{"minutes":"30"}`},
		{"absent", "wc1", `{}`},
		{"corps invalide", "wc1", `nope`},
		{"id inconnu", "salon", `{"minutes":30}`},
		{"cle brute", "556", `{"minutes":30}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newLTEnv(t)
			e.userRows("a@example.com", "admin_local", 3)
			e.userRows("a@example.com", "admin_local", 3) // admin sub-group session check
			e.profileRows("a@example.com", "admin_local", 3, true)
			rec := e.do(t, http.MethodPut, ltPath+c.id, "a@example.com", "admin_local", c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
			}
			wantErr := "invalid_minutes"
			if c.id == "salon" || c.id == "556" {
				wantErr = "unknown_light"
			}
			if !strings.Contains(rec.Body.String(), `"error":"`+wantErr+`"`) {
				t.Fatalf("body %s, want error %s", rec.Body.String(), wantErr)
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLightTimersPut_bornes0et255(t *testing.T) {
	for _, body := range []string{`{"minutes":0}`, `{"minutes":255}`} {
		e := newLTEnv(t)
		e.userRows("a@example.com", "admin_local", 3)
		e.userRows("a@example.com", "admin_local", 3) // admin sub-group session check
		e.profileRows("a@example.com", "admin_local", 3, true)
		e.expectWriteChain(3)
		rec := e.do(t, http.MethodPut, ltPath+"dressing", "a@example.com", "admin_local", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", body, rec.Code, rec.Body.String())
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLightTimersPut_lienNonApprouveRefuse(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("a@example.com", "admin_local", 3)
	e.userRows("a@example.com", "admin_local", 3) // admin sub-group session check
	e.profileRows("a@example.com", "admin_local", 3, false)
	rec := e.do(t, http.MethodPut, ltPath+"wc2", "a@example.com", "admin_local", `{"minutes":30}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"error":"forbidden"`) {
		t.Fatalf("status %d body %s, want 403 forbidden", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightTimersPut_dryRunSansAction(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("a@example.com", "admin_local", 3)
	e.userRows("a@example.com", "admin_local", 3) // admin sub-group session check
	e.profileRows("a@example.com", "admin_local", 3, true)
	rec := e.do(t, http.MethodPut, ltPath+"ann1", "a@example.com", "admin_local", `{"minutes":15}`, "X-Essensys-Test-Mode", "dry-run")
	if !strings.Contains(rec.Body.String(), `"status":"dry_run"`) {
		t.Fatalf("not a dry-run answer: %s", rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Validated []struct {
			K int    `json:"k"`
			V string `json:"v"`
		} `json:"validated_params"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Validated) != 1 || out.Validated[0].K != 561 || out.Validated[0].V != "15" {
		t.Fatalf("validated params: %+v err=%v", out, err)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("dry-run must not write: %v", err)
	}
}

func TestLightTimersGet_valeurConnueEtNull(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.com", "user", 3)
	e.profileRows("u@example.com", "user", 3, true)
	e.mock.ExpectQuery(`SELECT machine_id, keys, updated_at FROM gateway_exchange_cache`).
		WillReturnRows(sqlmock.NewRows([]string{"machine_id", "keys", "updated_at"}).
			AddRow(3, []byte(`[{"k":555,"v":"15"},{"k":556,"v":"0"}]`), time.Now()))
	rec := e.do(t, http.MethodGet, "/api/portal/lights/timers", "u@example.com", "user", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Timers []struct {
			ID      string `json:"id"`
			Label   string `json:"label"`
			Key     int    `json:"key"`
			Minutes *int   `json:"minutes"`
		} `json:"timers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Timers) != 6 {
		t.Fatalf("%d timers", len(out.Timers))
	}
	order := []string{"dressing", "wc1", "wc2", "pieceserv", "ann1", "ann2"}
	for i, id := range order {
		if out.Timers[i].ID != id {
			t.Fatalf("order[%d] = %s, want %s", i, out.Timers[i].ID, id)
		}
	}
	if out.Timers[3].Label != "Service" {
		t.Fatalf("label %q", out.Timers[3].Label)
	}
	for _, tm := range out.Timers {
		switch tm.ID {
		case "wc1":
			if tm.Minutes == nil || *tm.Minutes != 15 {
				t.Fatalf("wc1: %v", tm.Minutes)
			}
		case "wc2":
			if tm.Minutes == nil || *tm.Minutes != 0 {
				t.Fatalf("wc2 explicit 0: %v", tm.Minutes)
			}
		default:
			if tm.Minutes != nil {
				t.Fatalf("%s must be null, got %d", tm.ID, *tm.Minutes)
			}
		}
	}
	if !strings.Contains(rec.Body.String(), `"minutes":null`) {
		t.Fatalf("null must be explicit in JSON: %s", rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightTimersGet_cacheAbsentToutNull(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.com", "user", 3)
	e.profileRows("u@example.com", "user", 3, true)
	e.mock.ExpectQuery(`SELECT machine_id, keys, updated_at FROM gateway_exchange_cache`).WillReturnError(errNoRows)
	rec := e.do(t, http.MethodGet, "/api/portal/lights/timers", "u@example.com", "user", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if n := strings.Count(rec.Body.String(), `"minutes":null`); n != 6 {
		t.Fatalf("want 6 null, got %d: %s", n, rec.Body.String())
	}
}

func TestLightTimersGet_sansArmoireLiee403(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.com", "user", nil)
	e.profileRows("u@example.com", "user", nil, false)
	rec := e.do(t, http.MethodGet, "/api/portal/lights/timers", "u@example.com", "user", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestLightTimers_sansJetonRefuse(t *testing.T) {
	e := newLTEnv(t)
	for _, c := range []struct{ m, p string }{{"GET", "/api/portal/lights/timers"}, {"PUT", ltPath + "wc1"}} {
		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, httptest.NewRequest(c.m, c.p, strings.NewReader(`{"minutes":1}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d, want 401", c.m, c.p, rec.Code)
		}
	}
}

// NR: NR-backend-14 essensys-hub/essensys-user-portal-backend#53
// Non-regression: an administrator can never be bypassed. Whatever the body,
// a role that is not admin_local/admin_global (with a linked cabinet) gets 403
// and no action reaches the queue, so nobody else can alter a light delay.
func TestNR_backend_14_temporisation_jamais_ecrite_par_un_non_admin(t *testing.T) {
	for _, role := range []string{"user", "guest_local", "support"} {
		e := newLTEnv(t)
		for _, body := range []string{`{"minutes":30}`, `{"minutes":256}`, `x`} {
			e.userRows("s@example.com", role, 3)
			if role == "support" {
				// support is refused (403) before the body is even read.
				e.userRows("s@example.com", role, 3)
				e.profileRows("s@example.com", role, 3, true)
			}
			rec := e.do(t, http.MethodPut, ltPath+"dressing", "s@example.com", role, body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status %d, want 403", role, body, rec.Code)
			}
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s: %v", role, err)
		}
	}
}

// jsonArg matches a JSON column whether the driver gets it as []byte or string.
type jsonArg string

func (j jsonArg) Match(v driver.Value) bool {
	switch x := v.(type) {
	case []byte:
		return string(x) == string(j)
	case string:
		return x == string(j)
	}
	return false
}
