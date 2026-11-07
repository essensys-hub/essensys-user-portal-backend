package api

import (
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

// noms-lumieres-2026-11-004 (#55): custom light names routes, through the real
// router, with a mocked database. Nothing here touches a real cabinet.

const (
	lnPath  = "/api/portal/admin/lights/names/"
	lnGet   = "/api/portal/lights/names"
	lnSync  = "/api/gateway/lights/names/sync"
	lnToken = "gw-token-for-tests"
)

var lnCols = []string{"light_id", "name", "version", "updated_at", "updated_by"}

type lnView struct {
	ID        string  `json:"id"`
	Default   string  `json:"default"`
	Name      *string `json:"name"`
	Version   int     `json:"version"`
	UpdatedAt *string `json:"updated_at"`
	Status    string  `json:"status"`
}

func (e *ltEnv) adminWriteSetup(email string) {
	e.userRows(email, "admin_local", 3)
	e.userRows(email, "admin_local", 3) // admin sub-group session check
	e.profileRows(email, "admin_local", 3, true)
}

func TestLightNamesGet_33AvecNullParDefaut(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", 3)
	e.profileRows("u@example.org", "user", 3, true)
	e.mock.ExpectQuery(`FROM light_names WHERE machine_id = \$1`).WithArgs(3).WillReturnRows(sqlmock.NewRows(lnCols))
	rec := e.do(t, http.MethodGet, lnGet, "u@example.org", "user", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Lights []lnView `json:"lights"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Lights) != 33 {
		t.Fatalf("%d lights, err %v", len(out.Lights), err)
	}
	if out.Lights[0].ID != "terrasse" || out.Lights[20].ID != "dressing" || out.Lights[32].ID != "idressing" {
		t.Fatalf("order: %s %s %s", out.Lights[0].ID, out.Lights[20].ID, out.Lights[32].ID)
	}
	for _, l := range out.Lights {
		if l.Name != nil || l.Version != 0 || l.UpdatedAt != nil || l.Default == "" {
			t.Fatalf("%s: %+v", l.ID, l)
		}
	}
	if n := strings.Count(rec.Body.String(), `"name":null`); n != 33 {
		t.Fatalf("name null must be explicit 33 times, got %d", n)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesGet_nomPersonnaliseEtRemiseAZero(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", 3)
	e.profileRows("u@example.org", "user", 3, true)
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).
		AddRow("dressing", "WC 3", 2, at, "a@example.org").
		AddRow("wc1", nil, 3, at, "a@example.org"))
	rec := e.do(t, http.MethodGet, lnGet, "u@example.org", "user", "")
	var out struct {
		Lights []lnView `json:"lights"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, l := range out.Lights {
		switch l.ID {
		case "dressing":
			if l.Name == nil || *l.Name != "WC 3" || l.Version != 2 || l.UpdatedAt == nil || *l.UpdatedAt != "2026-11-07T10:15:30.123Z" || l.Default != "Dressing" {
				t.Fatalf("dressing: %+v", l)
			}
		case "wc1":
			if l.Name != nil || l.Version != 3 {
				t.Fatalf("wc1 reset keeps its version: %+v", l)
			}
		}
	}
	if strings.Contains(rec.Body.String(), "updated_by") || strings.Contains(rec.Body.String(), "a@example.org") {
		t.Fatal("the author must not be exposed to readers")
	}
}

func TestLightNamesGet_sansArmoireLiee403(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", nil)
	e.profileRows("u@example.org", "user", nil, false)
	rec := e.do(t, http.MethodGet, lnGet, "u@example.org", "user", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestLightNamesPut_adminRenommeAvecAudit(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	at := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM light_names WHERE machine_id`).WithArgs(3).
		WillReturnRows(sqlmock.NewRows(lnCols).AddRow("dressing", "Ancien", 1, at, "a@example.org"))
	e.mock.ExpectQuery(`INSERT INTO light_names`).WithArgs(3, "dressing", "WC 3", sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(lnCols).AddRow("dressing", "WC 3", 2, time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.org", "light_name", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, lnPath+"dressing", "a@example.org", "admin_local", `{"name":"  WC 3 "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out lnView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID != "dressing" || out.Name == nil || *out.Name != "WC 3" || out.Version != 2 || out.Status != "saved" || out.Default != "Dressing" {
		t.Fatalf("contract body: %s (%v)", rec.Body.String(), err)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesPut_auditContientAncienEtNouveauNom(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	at := time.Now()
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", "Ancien", 1, at, "x"))
	e.mock.ExpectQuery(`INSERT INTO light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", "Nouveau", 2, at, "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.org", "light_name", jsonContains{`"old_name":"Ancien"`, `"new_name":"Nouveau"`, `"id":"wc1"`}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, lnPath+"wc1", "a@example.org", "admin_local", `{"name":"Nouveau"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesPut_remiseAZero(t *testing.T) {
	for _, body := range []string{`{"name":null}`, `{"name":""}`, `{"name":"   "}`} {
		e := newLTEnv(t)
		e.adminWriteSetup("a@example.org")
		at := time.Now()
		e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", "Ancien", 1, at, "x"))
		e.mock.ExpectQuery(`INSERT INTO light_names`).WithArgs(3, "wc1", nil, sqlmock.AnyArg(), "a@example.org").
			WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", nil, 2, at, "a@example.org"))
		e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
		rec := e.do(t, http.MethodPut, lnPath+"wc1", "a@example.org", "admin_local", body)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":null`) || !strings.Contains(rec.Body.String(), `"version":2`) {
			t.Fatalf("%s: status %d: %s", body, rec.Code, rec.Body.String())
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
	}
}

func TestLightNamesPut_rolesEtArmoire(t *testing.T) {
	cases := []struct {
		name    string
		role    string
		machine any
		want    int
		wantExe bool
	}{
		{"admin_local de l'armoire", "admin_local", 3, http.StatusOK, true},
		{"admin_global avec armoire liee", "admin_global", 3, http.StatusOK, true},
		{"admin_global sans armoire", "admin_global", nil, http.StatusForbidden, false},
		{"admin_local sans armoire (autre armoire)", "admin_local", nil, http.StatusForbidden, false},
		{"support", "support", 3, http.StatusForbidden, false},
		{"user", "user", 3, http.StatusForbidden, false},
		{"guest_local", "guest_local", 3, http.StatusForbidden, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newLTEnv(t)
			email := "x@example.org"
			e.userRows(email, c.role, c.machine)
			if c.role != "user" && c.role != "guest_local" {
				e.userRows(email, c.role, c.machine)
				e.profileRows(email, c.role, c.machine, true)
				if c.wantExe {
					e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols))
					e.mock.ExpectQuery(`INSERT INTO light_names`).WithArgs(c.machine.(int), "salon", "Séjour", sqlmock.AnyArg(), email).
						WillReturnRows(sqlmock.NewRows(lnCols).AddRow("salon", "Séjour", 1, time.Now(), email))
					e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
				}
			}
			rec := e.do(t, http.MethodPut, lnPath+"salon", email, c.role, `{"name":"Séjour"}`)
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet (a write is missing) or unexpected: %v", err)
			}
		})
	}
}

func TestLightNamesPut_validation400SansEcriture(t *testing.T) {
	cases := []struct{ name, id, body, want string }{
		{"41 caracteres", "wc1", `{"name":"` + strings.Repeat("a", 41) + `"}`, "invalid_name"},
		{"balise", "wc1", `{"name":"<b>x</b>"}`, "invalid_name"},
		{"chevron", "wc1", `{"name":"a>b"}`, "invalid_name"},
		{"controle", "wc1", `{"name":"a\u0000b"}`, "invalid_name"},
		{"nombre", "wc1", `{"name":5}`, "invalid_name"},
		{"cle absente", "wc1", `{}`, "invalid_name"},
		{"corps invalide", "wc1", `nope`, "invalid_name"},
		{"id inconnu", "salon2", `{"name":"ok"}`, "unknown_light"},
		{"cle d'echange", "556", `{"name":"ok"}`, "unknown_light"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newLTEnv(t)
			e.adminWriteSetup("a@example.org")
			rec := e.do(t, http.MethodPut, lnPath+c.id, "a@example.org", "admin_local", c.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"`+c.want+`"`) {
				t.Fatalf("status %d body %s, want 400 %s", rec.Code, rec.Body.String(), c.want)
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLightNamesPut_40CaracteresAcceptes(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	name := strings.Repeat("é", 40)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols))
	e.mock.ExpectQuery(`INSERT INTO light_names`).WithArgs(3, "wc1", name, sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", name, 1, time.Now(), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, lnPath+"wc1", "a@example.org", "admin_local", `{"name":"`+name+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesPut_dryRunSansEcriture(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	rec := e.do(t, http.MethodPut, lnPath+"dressing", "a@example.org", "admin_local", `{"name":"WC 3"}`, "X-Essensys-Test-Mode", "dry-run")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"dry_run"`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("dry-run must not touch the table: %v", err)
	}
}

func TestLightNames_sansJetonRefuse(t *testing.T) {
	e := newLTEnv(t)
	for _, c := range []struct{ m, p string }{{"GET", lnGet}, {"PUT", lnPath + "wc1"}} {
		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, httptest.NewRequest(c.m, c.p, strings.NewReader(`{"name":"x"}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d, want 401", c.m, c.p, rec.Code)
		}
	}
}

// --- gateway synchronisation -------------------------------------------------

func (e *ltEnv) gatewaySetup(machine int) {
	sum := sha256.Sum256([]byte(lnToken))
	e.mock.ExpectQuery(`FROM gateway_sessions WHERE gateway_id`).WithArgs("gw-1").
		WillReturnRows(sqlmock.NewRows([]string{"gateway_id", "token_hash", "machine_id", "eth0_mac", "eth1_mac", "last_seen"}).
			AddRow("gw-1", hex.EncodeToString(sum[:]), machine, nil, nil, nil))
	e.mock.ExpectQuery(`SELECT machine_id FROM gateway_sessions`).WithArgs("gw-1").
		WillReturnRows(sqlmock.NewRows([]string{"machine_id"}).AddRow(machine))
}

func (e *ltEnv) sync(body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, lnSync, strings.NewReader(body))
	if auth {
		req.Header.Set("X-Gateway-ID", "gw-1")
		req.Header.Set("Authorization", "Bearer "+lnToken)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func lnState(t *testing.T, rec *httptest.ResponseRecorder, id string) lnView {
	t.Helper()
	var out struct {
		Lights []lnView `json:"lights"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Lights) != 33 {
		t.Fatalf("response: %d lights, err %v: %s", len(out.Lights), err, rec.Body.String())
	}
	for _, l := range out.Lights {
		if l.ID == id {
			return l
		}
	}
	t.Fatalf("%s missing", id)
	return lnView{}
}

func TestLightNamesSync_cloudPlusRecentGagne(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	cloudAt := time.Date(2026, 11, 7, 10, 15, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM light_names`).WithArgs(3).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("dressing", "WC 3", 2, cloudAt, "a@example.org"))
	// No write expected: the cloud keeps its value.
	rec := e.sync(`{"names":[{"id":"dressing","name":"Dressing 2","version":9,"updated_at":"2026-11-07T10:10:00.000Z"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if l := lnState(t, rec, "dressing"); l.Name == nil || *l.Name != "WC 3" || l.Version != 2 {
		t.Fatalf("cloud must win: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("the cloud must not be modified: %v", err)
	}
}

func TestLightNamesSync_gatewayPlusRecenteGagne(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", "Vieux", 1, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
	gwAt := time.Date(2026, 11, 7, 10, 20, 0, 0, time.UTC)
	e.mock.ExpectExec(`INSERT INTO light_names`).WithArgs(3, "wc1", "WC Haut", 2, gwAt, "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("gateway:gw-1", "light_name", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.sync(`{"names":[{"id":"wc1","name":"WC Haut","version":2,"updated_at":"2026-11-07T10:20:00.000Z"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if l := lnState(t, rec, "wc1"); l.Name == nil || *l.Name != "WC Haut" || l.Version != 2 {
		t.Fatalf("gateway must win: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesSync_egaliteLeCloudReste(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	at := time.Date(2026, 11, 7, 10, 15, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", "Cloud", 2, at, "a@example.org"))
	rec := e.sync(`{"names":[{"id":"wc1","name":"Gateway","version":2,"updated_at":"2026-11-07T10:15:00.000Z","updated_by":"zzz@example.org"}]}`, true)
	if l := lnState(t, rec, "wc1"); l.Name == nil || *l.Name != "Cloud" {
		t.Fatalf("tie: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesSync_premierContactEnvoieSesNoms(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols))
	e.mock.ExpectExec(`INSERT INTO light_names`).WithArgs(3, "salon", "Séjour", 1, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.sync(`{"names":[{"id":"salon","name":"Séjour","version":1,"updated_at":"2026-11-07T10:20:00.000Z"}]}`, true)
	if l := lnState(t, rec, "salon"); l.Name == nil || *l.Name != "Séjour" {
		t.Fatalf("%+v", l)
	}
}

func TestLightNamesSync_remiseAZeroSePropage(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", "Nom", 1, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO light_names`).WithArgs(3, "wc1", nil, 2, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.sync(`{"names":[{"id":"wc1","name":null,"version":2,"updated_at":"2026-11-07T10:30:00.000Z"}]}`, true)
	if l := lnState(t, rec, "wc1"); l.Name != nil || l.Version != 2 {
		t.Fatalf("reset must win: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesSync_horlogeDansLeFuturRamenee(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols))
	e.mock.ExpectExec(`INSERT INTO light_names`).WithArgs(3, "wc1", "Futur", 1, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	future := time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	rec := e.sync(`{"names":[{"id":"wc1","name":"Futur","version":1,"updated_at":"`+future+`"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	l := lnState(t, rec, "wc1")
	if l.UpdatedAt == nil || *l.UpdatedAt >= future {
		t.Fatalf("future timestamp must be brought back to the cloud clock: %+v", l)
	}
	if at, err := time.Parse(time.RFC3339Nano, *l.UpdatedAt); err != nil || time.Since(at) > 5*time.Second || time.Until(at) > time.Second {
		t.Fatalf("brought back timestamp must be about now: %v (%v)", *l.UpdatedAt, err)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesSync_entreesInvalidesIgnorees(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols))
	// Only the valid one is written.
	e.mock.ExpectExec(`INSERT INTO light_names`).WithArgs(3, "sam", "Repas", 1, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.sync(`{"names":[
		{"id":"inconnue","name":"x","version":1,"updated_at":"2026-11-07T10:20:00.000Z"},
		{"id":"wc1","name":"<b>","version":1,"updated_at":"2026-11-07T10:20:00.000Z"},
		{"id":"wc2","name":"ok","version":1,"updated_at":"pas une date"},
		{"id":"salon","name":null,"version":0,"updated_at":null},
		{"id":"sam","name":"Repas","version":1,"updated_at":"2026-11-07T10:20:00.000Z"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesSync_corpsInvalideEtTropLong400(t *testing.T) {
	var many strings.Builder
	many.WriteString(`{"names":[`)
	for i := 0; i < 34; i++ {
		if i > 0 {
			many.WriteString(",")
		}
		many.WriteString(`{"id":"wc1","name":"x","version":1,"updated_at":"2026-11-07T10:20:00.000Z"}`)
	}
	many.WriteString(`]}`)
	for _, body := range []string{`nope`, many.String()} {
		e := newLTEnv(t)
		e.gatewaySetup(3)
		rec := e.sync(body, true)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400: %.80s", rec.Code, rec.Body.String())
		}
	}
}

func TestLightNamesSync_videRenvoieEtatComplet(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("wc1", "Nom", 1, time.Now(), "a@example.org"))
	rec := e.sync(`{"names":[]}`, true)
	if rec.Code != http.StatusOK || lnState(t, rec, "wc1").Name == nil {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLightNamesSync_sansJeton401(t *testing.T) {
	e := newLTEnv(t)
	rec := e.sync(`{"names":[]}`, false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// NR: NR-backend-15 essensys-hub/essensys-user-portal-backend#55 (ticket parent essensys-feature-lifecycle#41)
// Non-regression: a custom name can never be written by someone who is not the
// administrator of the cabinet, nor by an unauthenticated gateway, whatever the
// body. The legacy exchange table is never involved.
func TestNR_backend_15_nom_jamais_ecrit_par_un_non_admin(t *testing.T) {
	for _, role := range []string{"user", "guest_local", "support"} {
		e := newLTEnv(t)
		for _, body := range []string{`{"name":"Pirate"}`, `{"name":null}`, `x`} {
			e.userRows("s@example.org", role, 3)
			if role == "support" {
				e.userRows("s@example.org", role, 3)
				e.profileRows("s@example.org", role, 3, true)
			}
			rec := e.do(t, http.MethodPut, lnPath+"dressing", "s@example.org", role, body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status %d, want 403", role, body, rec.Code)
			}
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s: %v", role, err)
		}
	}
	e := newLTEnv(t)
	if rec := e.sync(`{"names":[{"id":"wc1","name":"Pirate","version":99,"updated_at":"2999-01-01T00:00:00.000Z"}]}`, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated gateway: %d, want 401", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// jsonContains matches a JSON column that holds every given fragment.
type jsonContains []string

func (j jsonContains) Match(v driver.Value) bool {
	var s string
	switch x := v.(type) {
	case []byte:
		s = string(x)
	case string:
		s = x
	default:
		return false
	}
	for _, f := range j {
		if !strings.Contains(s, f) {
			return false
		}
	}
	return true
}

func TestLightNamesSync_entreeVierge33NEffacePasLeCloud(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("dressing", "WC 3", 2, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
	var b strings.Builder
	b.WriteString(`{"names":[`)
	for i, l := range lnIDs() {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"` + l + `","name":null,"version":0,"updated_at":null}`)
	}
	b.WriteString(`]}`)
	rec := e.sync(b.String(), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if l := lnState(t, rec, "dressing"); l.Name == nil || *l.Name != "WC 3" {
		t.Fatalf("blank entries must not erase the cloud: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("no write expected: %v", err)
	}
}

func TestLightNamesSync_updatedAtAbsentIgnoree(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols))
	rec := e.sync(`{"names":[{"id":"wc1","name":"x","version":3}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLightNamesSync_reponseToujours33DansLOrdre(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM light_names`).WillReturnRows(sqlmock.NewRows(lnCols).AddRow("idressing", "Placards", 1, time.Now(), "x"))
	rec := e.sync(`{"names":[]}`, true)
	var out struct {
		Lights []lnView `json:"lights"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Lights) != 33 {
		t.Fatalf("%d lights %v", len(out.Lights), err)
	}
	for i, id := range lnIDs() {
		if out.Lights[i].ID != id {
			t.Fatalf("[%d] %s want %s", i, out.Lights[i].ID, id)
		}
	}
}

func lnIDs() []string {
	var ids []string
	for _, e := range domain.LightNameCatalog() {
		ids = append(ids, e.ID)
	}
	return ids
}
