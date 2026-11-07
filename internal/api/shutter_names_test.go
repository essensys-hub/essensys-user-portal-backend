package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

// noms-volets-2026-11-005 (#57): custom shutter names routes, through the real
// router, with a mocked database. Nothing here touches a real cabinet.

const (
	snPath = "/api/portal/admin/shutters/names/"
	snGet  = "/api/portal/shutters/names"
	snSync = "/api/gateway/shutters/names/sync"
)

var snCols = []string{"shutter_id", "name", "version", "updated_at", "updated_by"}

type snView struct {
	ID        string  `json:"id"`
	Default   string  `json:"default"`
	Name      *string `json:"name"`
	Version   int     `json:"version"`
	UpdatedAt *string `json:"updated_at"`
	Status    string  `json:"status"`
}

func TestShutterNamesGet_16AvecNullParDefaut(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", 3)
	e.profileRows("u@example.org", "user", 3, true)
	e.mock.ExpectQuery(`FROM shutter_names WHERE machine_id = \$1`).WithArgs(3).WillReturnRows(sqlmock.NewRows(snCols))
	rec := e.do(t, http.MethodGet, snGet, "u@example.org", "user", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Shutters []snView `json:"shutters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Shutters) != 16 {
		t.Fatalf("%d shutters, err %v", len(out.Shutters), err)
	}
	if out.Shutters[0].ID != "volet1salon" || out.Shutters[7].ID != "voletsdb" || out.Shutters[15].ID != "store" {
		t.Fatalf("order: %s %s %s", out.Shutters[0].ID, out.Shutters[7].ID, out.Shutters[15].ID)
	}
	for _, l := range out.Shutters {
		if l.Name != nil || l.Version != 0 || l.UpdatedAt != nil || l.Default == "" {
			t.Fatalf("%s: %+v", l.ID, l)
		}
	}
	if n := strings.Count(rec.Body.String(), `"name":null`); n != 16 {
		t.Fatalf("name null must be explicit 16 times, got %d", n)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesGet_nomPersonnaliseEtRemiseAZero(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", 3)
	e.profileRows("u@example.org", "user", 3, true)
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).
		AddRow("volet1salon", "Volet baie vitrée", 2, at, "a@example.org").
		AddRow("voletstore", nil, 3, at, "a@example.org"))
	rec := e.do(t, http.MethodGet, snGet, "u@example.org", "user", "")
	var out struct {
		Shutters []snView `json:"shutters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, l := range out.Shutters {
		switch l.ID {
		case "volet1salon":
			if l.Name == nil || *l.Name != "Volet baie vitrée" || l.Version != 2 || l.UpdatedAt == nil || *l.UpdatedAt != "2026-11-07T10:15:30.123Z" || l.Default != "Volet 1 Salon" {
				t.Fatalf("volet1salon: %+v", l)
			}
		case "voletstore":
			if l.Name != nil || l.Version != 3 {
				t.Fatalf("voletstore reset keeps its version: %+v", l)
			}
		}
	}
	if strings.Contains(rec.Body.String(), "updated_by") || strings.Contains(rec.Body.String(), "a@example.org") {
		t.Fatal("the author must not be exposed to readers")
	}
}

func TestShutterNamesGet_sansArmoireLiee403(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", nil)
	e.profileRows("u@example.org", "user", nil, false)
	rec := e.do(t, http.MethodGet, snGet, "u@example.org", "user", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestShutterNamesPut_adminRenommeAvecAudit(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	at := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM shutter_names WHERE machine_id`).WithArgs(3).
		WillReturnRows(sqlmock.NewRows(snCols).AddRow("volet1salon", "Ancien", 1, at, "a@example.org"))
	e.mock.ExpectQuery(`INSERT INTO shutter_names`).WithArgs(3, "volet1salon", "Volet baie vitrée", sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(snCols).AddRow("volet1salon", "Volet baie vitrée", 2, time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.org", "shutter_name", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, snPath+"volet1salon", "a@example.org", "admin_local", `{"name":"  Volet baie vitrée "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out snView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID != "volet1salon" || out.Name == nil || *out.Name != "Volet baie vitrée" || out.Version != 2 || out.Status != "saved" || out.Default != "Volet 1 Salon" {
		t.Fatalf("contract body: %s (%v)", rec.Body.String(), err)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesPut_auditContientAncienEtNouveauNom(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	at := time.Now()
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", "Ancien", 1, at, "x"))
	e.mock.ExpectQuery(`INSERT INTO shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", "Nouveau", 2, at, "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.org", "shutter_name", jsonContains{`"old_name":"Ancien"`, `"new_name":"Nouveau"`, `"id":"voletstore"`}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, snPath+"voletstore", "a@example.org", "admin_local", `{"name":"Nouveau"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesPut_remiseAZero(t *testing.T) {
	for _, body := range []string{`{"name":null}`, `{"name":""}`, `{"name":"   "}`} {
		e := newLTEnv(t)
		e.adminWriteSetup("a@example.org")
		at := time.Now()
		e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", "Ancien", 1, at, "x"))
		e.mock.ExpectQuery(`INSERT INTO shutter_names`).WithArgs(3, "voletstore", nil, sqlmock.AnyArg(), "a@example.org").
			WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", nil, 2, at, "a@example.org"))
		e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
		rec := e.do(t, http.MethodPut, snPath+"voletstore", "a@example.org", "admin_local", body)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":null`) || !strings.Contains(rec.Body.String(), `"version":2`) {
			t.Fatalf("%s: status %d: %s", body, rec.Code, rec.Body.String())
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
	}
}

func TestShutterNamesPut_rolesEtArmoire(t *testing.T) {
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
					e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols))
					e.mock.ExpectQuery(`INSERT INTO shutter_names`).WithArgs(c.machine.(int), "volet2salon", "Séjour", sqlmock.AnyArg(), email).
						WillReturnRows(sqlmock.NewRows(snCols).AddRow("volet2salon", "Séjour", 1, time.Now(), email))
					e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
				}
			}
			rec := e.do(t, http.MethodPut, snPath+"volet2salon", email, c.role, `{"name":"Séjour"}`)
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet (a write is missing) or unexpected: %v", err)
			}
		})
	}
}

func TestShutterNamesPut_validation400SansEcriture(t *testing.T) {
	cases := []struct{ name, id, body, want string }{
		{"41 caracteres", "voletstore", `{"name":"` + strings.Repeat("a", 41) + `"}`, "invalid_name"},
		{"balise", "voletstore", `{"name":"<b>x</b>"}`, "invalid_name"},
		{"chevron", "voletstore", `{"name":"a>b"}`, "invalid_name"},
		{"controle", "voletstore", `{"name":"a\u0000b"}`, "invalid_name"},
		{"nombre", "voletstore", `{"name":5}`, "invalid_name"},
		{"cle absente", "voletstore", `{}`, "invalid_name"},
		{"corps invalide", "voletstore", `nope`, "invalid_name"},
		{"id inconnu", "volet99", `{"name":"ok"}`, "unknown_shutter"},
		{"id de lumiere", "dressing", `{"name":"ok"}`, "unknown_shutter"},
		{"cle d'echange", "556", `{"name":"ok"}`, "unknown_shutter"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newLTEnv(t)
			e.adminWriteSetup("a@example.org")
			rec := e.do(t, http.MethodPut, snPath+c.id, "a@example.org", "admin_local", c.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"`+c.want+`"`) {
				t.Fatalf("status %d body %s, want 400 %s", rec.Code, rec.Body.String(), c.want)
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestShutterNamesPut_40CaracteresAcceptes(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	name := strings.Repeat("é", 40)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols))
	e.mock.ExpectQuery(`INSERT INTO shutter_names`).WithArgs(3, "voletstore", name, sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", name, 1, time.Now(), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, snPath+"voletstore", "a@example.org", "admin_local", `{"name":"`+name+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesPut_dryRunSansEcriture(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	rec := e.do(t, http.MethodPut, snPath+"volet1salon", "a@example.org", "admin_local", `{"name":"Volet baie vitrée"}`, "X-Essensys-Test-Mode", "dry-run")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"dry_run"`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("dry-run must not touch the table: %v", err)
	}
}

func TestShutterNames_sansJetonRefuse(t *testing.T) {
	e := newLTEnv(t)
	for _, c := range []struct{ m, p string }{{"GET", snGet}, {"PUT", snPath + "voletstore"}} {
		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, httptest.NewRequest(c.m, c.p, strings.NewReader(`{"name":"x"}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d, want 401", c.m, c.p, rec.Code)
		}
	}
}

// --- gateway synchronisation -------------------------------------------------

func (e *ltEnv) syncShutters(body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, snSync, strings.NewReader(body))
	if auth {
		req.Header.Set("X-Gateway-ID", "gw-1")
		req.Header.Set("Authorization", "Bearer "+lnToken)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func snState(t *testing.T, rec *httptest.ResponseRecorder, id string) snView {
	t.Helper()
	var out struct {
		Shutters []snView `json:"shutters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Shutters) != 16 {
		t.Fatalf("response: %d shutters, err %v: %s", len(out.Shutters), err, rec.Body.String())
	}
	for _, l := range out.Shutters {
		if l.ID == id {
			return l
		}
	}
	t.Fatalf("%s missing", id)
	return snView{}
}

func TestShutterNamesSync_cloudPlusRecentGagne(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	cloudAt := time.Date(2026, 11, 7, 10, 15, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM shutter_names`).WithArgs(3).WillReturnRows(sqlmock.NewRows(snCols).AddRow("volet1salon", "Volet baie vitrée", 2, cloudAt, "a@example.org"))
	// No write expected: the cloud keeps its value.
	rec := e.syncShutters(`{"names":[{"id":"volet1salon","name":"Volet 2","version":9,"updated_at":"2026-11-07T10:10:00.000Z"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if l := snState(t, rec, "volet1salon"); l.Name == nil || *l.Name != "Volet baie vitrée" || l.Version != 2 {
		t.Fatalf("cloud must win: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("the cloud must not be modified: %v", err)
	}
}

func TestShutterNamesSync_gatewayPlusRecenteGagne(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", "Vieux", 1, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
	gwAt := time.Date(2026, 11, 7, 10, 20, 0, 0, time.UTC)
	e.mock.ExpectExec(`INSERT INTO shutter_names`).WithArgs(3, "voletstore", "Store terrasse", 2, gwAt, "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("gateway:gw-1", "shutter_name", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.syncShutters(`{"names":[{"id":"voletstore","name":"Store terrasse","version":2,"updated_at":"2026-11-07T10:20:00.000Z"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if l := snState(t, rec, "voletstore"); l.Name == nil || *l.Name != "Store terrasse" || l.Version != 2 {
		t.Fatalf("gateway must win: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesSync_egaliteLeCloudReste(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	at := time.Date(2026, 11, 7, 10, 15, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", "Cloud", 2, at, "a@example.org"))
	rec := e.syncShutters(`{"names":[{"id":"voletstore","name":"Gateway","version":2,"updated_at":"2026-11-07T10:15:00.000Z","updated_by":"zzz@example.org"}]}`, true)
	if l := snState(t, rec, "voletstore"); l.Name == nil || *l.Name != "Cloud" {
		t.Fatalf("tie: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesSync_premierContactEnvoieSesNoms(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols))
	e.mock.ExpectExec(`INSERT INTO shutter_names`).WithArgs(3, "volet2salon", "Séjour", 1, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.syncShutters(`{"names":[{"id":"volet2salon","name":"Séjour","version":1,"updated_at":"2026-11-07T10:20:00.000Z"}]}`, true)
	if l := snState(t, rec, "volet2salon"); l.Name == nil || *l.Name != "Séjour" {
		t.Fatalf("%+v", l)
	}
}

func TestShutterNamesSync_remiseAZeroSePropage(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", "Nom", 1, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO shutter_names`).WithArgs(3, "voletstore", nil, 2, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.syncShutters(`{"names":[{"id":"voletstore","name":null,"version":2,"updated_at":"2026-11-07T10:30:00.000Z"}]}`, true)
	if l := snState(t, rec, "voletstore"); l.Name != nil || l.Version != 2 {
		t.Fatalf("reset must win: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesSync_horlogeDansLeFuturRamenee(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols))
	e.mock.ExpectExec(`INSERT INTO shutter_names`).WithArgs(3, "voletstore", "Futur", 1, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	future := time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	rec := e.syncShutters(`{"names":[{"id":"voletstore","name":"Futur","version":1,"updated_at":"`+future+`"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	l := snState(t, rec, "voletstore")
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

func TestShutterNamesSync_entreesInvalidesIgnorees(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols))
	// Only the valid one is written.
	e.mock.ExpectExec(`INSERT INTO shutter_names`).WithArgs(3, "volet3salon", "Repas", 1, sqlmock.AnyArg(), "gateway").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.syncShutters(`{"names":[
		{"id":"inconnue","name":"x","version":1,"updated_at":"2026-11-07T10:20:00.000Z"},
		{"id":"voletstore","name":"<b>","version":1,"updated_at":"2026-11-07T10:20:00.000Z"},
		{"id":"volet2cuisine","name":"ok","version":1,"updated_at":"pas une date"},
		{"id":"volet2salon","name":null,"version":0,"updated_at":null},
		{"id":"volet3salon","name":"Repas","version":1,"updated_at":"2026-11-07T10:20:00.000Z"}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesSync_corpsInvalideEtTropLong400(t *testing.T) {
	var many strings.Builder
	many.WriteString(`{"names":[`)
	for i := 0; i < 17; i++ {
		if i > 0 {
			many.WriteString(",")
		}
		many.WriteString(`{"id":"voletstore","name":"x","version":1,"updated_at":"2026-11-07T10:20:00.000Z"}`)
	}
	many.WriteString(`]}`)
	for _, body := range []string{`nope`, many.String()} {
		e := newLTEnv(t)
		e.gatewaySetup(3)
		rec := e.syncShutters(body, true)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400: %.80s", rec.Code, rec.Body.String())
		}
	}
}

func TestShutterNamesSync_videRenvoieEtatComplet(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("voletstore", "Nom", 1, time.Now(), "a@example.org"))
	rec := e.syncShutters(`{"names":[]}`, true)
	if rec.Code != http.StatusOK || snState(t, rec, "voletstore").Name == nil {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestShutterNamesSync_sansJeton401(t *testing.T) {
	e := newLTEnv(t)
	rec := e.syncShutters(`{"names":[]}`, false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Un non-administrateur ou une gateway sans jeton n'ecrit jamais un nom de volet.
func TestShutterNames_nomJamaisEcritParUnNonAdmin(t *testing.T) {
	for _, role := range []string{"user", "guest_local", "support"} {
		e := newLTEnv(t)
		for _, body := range []string{`{"name":"Pirate"}`, `{"name":null}`, `x`} {
			e.userRows("s@example.org", role, 3)
			if role == "support" {
				e.userRows("s@example.org", role, 3)
				e.profileRows("s@example.org", role, 3, true)
			}
			rec := e.do(t, http.MethodPut, snPath+"volet1salon", "s@example.org", role, body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status %d, want 403", role, body, rec.Code)
			}
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s: %v", role, err)
		}
	}
	e := newLTEnv(t)
	if rec := e.syncShutters(`{"names":[{"id":"voletstore","name":"Pirate","version":99,"updated_at":"2999-01-01T00:00:00.000Z"}]}`, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated gateway: %d, want 401", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesSync_entreeVierge16NEffacePasLeCloud(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("volet1salon", "Volet baie vitrée", 2, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
	var b strings.Builder
	b.WriteString(`{"names":[`)
	for i, l := range snIDs() {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"` + l + `","name":null,"version":0,"updated_at":null}`)
	}
	b.WriteString(`]}`)
	rec := e.syncShutters(b.String(), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if l := snState(t, rec, "volet1salon"); l.Name == nil || *l.Name != "Volet baie vitrée" {
		t.Fatalf("blank entries must not erase the cloud: %+v", l)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("no write expected: %v", err)
	}
}

func TestShutterNamesSync_updatedAtAbsentIgnoree(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols))
	rec := e.syncShutters(`{"names":[{"id":"voletstore","name":"x","version":3}]}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShutterNamesSync_reponseToujours16DansLOrdre(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM shutter_names`).WillReturnRows(sqlmock.NewRows(snCols).AddRow("store", "Placards", 1, time.Now(), "x"))
	rec := e.syncShutters(`{"names":[]}`, true)
	var out struct {
		Shutters []snView `json:"shutters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Shutters) != 16 {
		t.Fatalf("%d shutters %v", len(out.Shutters), err)
	}
	for i, id := range snIDs() {
		if out.Shutters[i].ID != id {
			t.Fatalf("[%d] %s want %s", i, out.Shutters[i].ID, id)
		}
	}
}

func snIDs() []string {
	var ids []string
	for _, e := range domain.ShutterNameCatalog() {
		ids = append(ids, e.ID)
	}
	return ids
}

// NR: NR-backend-16 essensys-hub/essensys-user-portal-backend#57 (ticket parent essensys-feature-lifecycle#41)
// Non-regression: renaming a shutter changes neither GET /portal/lights/names
// nor the light_names table. The mocked database accepts only shutter_names
// statements for the rename (any light_names query would fail the test), and
// the lights answer stays the one of #41 (33 lights, names untouched).
func TestNR_backend_16_renommer_un_volet_ne_change_pas_les_lumieres(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	e.mock.ExpectQuery(`FROM shutter_names WHERE machine_id`).WithArgs(3).WillReturnRows(sqlmock.NewRows(snCols))
	e.mock.ExpectQuery(`INSERT INTO shutter_names`).WithArgs(3, "volet1salon", "Volet baie vitrée", sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(snCols).AddRow("volet1salon", "Volet baie vitrée", 1, at, "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.org", "shutter_name", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	if rec := e.do(t, http.MethodPut, snPath+"volet1salon", "a@example.org", "admin_local", `{"name":"Volet baie vitrée"}`); rec.Code != http.StatusOK {
		t.Fatalf("rename: status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("the rename must only touch shutter_names: %v", err)
	}

	// The light answer is the one of #41, with the light stored data only.
	e2 := newLTEnv(t)
	e2.userRows("u@example.org", "user", 3)
	e2.profileRows("u@example.org", "user", 3, true)
	e2.mock.ExpectQuery(`FROM light_names WHERE machine_id = \$1`).WithArgs(3).
		WillReturnRows(sqlmock.NewRows([]string{"light_id", "name", "version", "updated_at", "updated_by"}).AddRow("dressing", "WC 3", 2, at, "a@example.org"))
	rec := e2.do(t, http.MethodGet, "/api/portal/lights/names", "u@example.org", "user", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "volet") || !strings.Contains(rec.Body.String(), `"WC 3"`) {
		t.Fatalf("lights answer changed: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Lights []json.RawMessage `json:"lights"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Lights) != 33 {
		t.Fatalf("lights must stay 33: %d %v", len(out.Lights), err)
	}
}
