package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// affichage-objets-2026-11-006 (#59) : routes de disposition des lumières et des
// volets, par le vrai routeur et une base simulée. Rien ici ne touche une armoire.

var olCols = []string{"order_json", "disabled_json", "version", "updated_at", "updated_by"}

type olKind struct {
	name    string // lights | shutters
	unknown string // code d'erreur d'identifiant inconnu
	other   string // identifiant de l'autre genre
	a, b, c string // identifiants du genre
}

var olKinds = []olKind{
	{"lights", "unknown_light", "volet1salon", "salon", "entree", "wc2"},
	{"shutters", "unknown_shutter", "salon", "volet2salon", "volet1salon", "store"},
}

func (k olKind) get() string  { return "/api/portal/" + k.name + "/layout" }
func (k olKind) put() string  { return "/api/portal/admin/" + k.name + "/layout" }
func (k olKind) sync() string { return "/api/gateway/" + k.name + "/layout/sync" }

type olView struct {
	Order     []string `json:"order"`
	Disabled  []string `json:"disabled"`
	Version   int      `json:"version"`
	UpdatedAt *string  `json:"updated_at"`
	UpdatedBy *string  `json:"updated_by"`
	Status    string   `json:"status"`
}

func olDecode(t *testing.T, rec *httptest.ResponseRecorder) olView {
	t.Helper()
	var v olView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%v : %s", err, rec.Body.String())
	}
	return v
}

func (e *ltEnv) olSync(k olKind, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, k.sync(), strings.NewReader(body))
	if auth {
		req.Header.Set("X-Gateway-ID", "gw-1")
		req.Header.Set("Authorization", "Bearer "+lnToken)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func TestLayoutGet_defautVersion0SansAuteur(t *testing.T) {
	for _, k := range olKinds {
		t.Run(k.name, func(t *testing.T) {
			e := newLTEnv(t)
			e.userRows("u@example.org", "user", 3)
			e.profileRows("u@example.org", "user", 3, true)
			e.mock.ExpectQuery(`FROM object_layout WHERE machine_id = \$1 AND kind = \$2`).WithArgs(3, k.name).WillReturnRows(sqlmock.NewRows(olCols))
			rec := e.do(t, http.MethodGet, k.get(), "u@example.org", "user", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"order":[],"disabled":[],"version":0,"updated_at":null}` {
				t.Fatalf("défaut attendu, reçu %s", got)
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLayoutGet_documentSansUpdatedBy(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", 3)
	e.profileRows("u@example.org", "user", 3, true)
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["salon","entree"]`, `["wc2"]`, 3, at, "a@example.org"))
	rec := e.do(t, http.MethodGet, olKinds[0].get(), "u@example.org", "user", "")
	v := olDecode(t, rec)
	if v.Version != 3 || len(v.Order) != 2 || v.Order[0] != "salon" || v.Disabled[0] != "wc2" || v.UpdatedAt == nil || *v.UpdatedAt != "2026-11-07T10:15:30.123Z" {
		t.Fatalf("%+v", v)
	}
	if strings.Contains(rec.Body.String(), "updated_by") || strings.Contains(rec.Body.String(), "a@example.org") {
		t.Fatal("l'auteur ne doit pas être exposé aux lecteurs")
	}
}

func TestLayoutGet_sansArmoireLiee403(t *testing.T) {
	for _, k := range olKinds {
		e := newLTEnv(t)
		e.userRows("u@example.org", "user", nil)
		e.profileRows("u@example.org", "user", nil, false)
		if rec := e.do(t, http.MethodGet, k.get(), "u@example.org", "user", ""); rec.Code != http.StatusForbidden {
			t.Fatalf("%s : status %d, attendu 403", k.name, rec.Code)
		}
	}
}

func TestLayoutGet_genresIndependants(t *testing.T) {
	// Une disposition de volets enregistrée ne change pas celle des lumières :
	// la lecture des lumières interroge kind = 'lights' et reçoit le défaut.
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", 3)
	e.profileRows("u@example.org", "user", 3, true)
	e.mock.ExpectQuery(`FROM object_layout`).WithArgs(3, "lights").WillReturnRows(sqlmock.NewRows(olCols))
	v := olDecode(t, e.do(t, http.MethodGet, olKinds[0].get(), "u@example.org", "user", ""))
	if v.Version != 0 || len(v.Order) != 0 {
		t.Fatalf("%+v", v)
	}
}

func TestLayoutGet_baieIndisponible503(t *testing.T) {
	e := newLTEnv(t)
	e.userRows("u@example.org", "user", 3)
	e.profileRows("u@example.org", "user", 3, true)
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnError(errNoRows2)
	rec := e.do(t, http.MethodGet, olKinds[0].get(), "u@example.org", "user", "")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"error":"names_unavailable"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

type olErr struct{}

func (olErr) Error() string { return "connexion perdue" }

var errNoRows2 = olErr{}

func TestLayoutPut_adminEnregistreAvecAudit(t *testing.T) {
	for _, k := range olKinds {
		t.Run(k.name, func(t *testing.T) {
			e := newLTEnv(t)
			e.adminWriteSetup("a@example.org")
			at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
			e.mock.ExpectQuery(`FROM object_layout WHERE machine_id`).WithArgs(3, k.name).
				WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["`+k.b+`"]`, `[]`, 1, at.Add(-time.Hour), "x"))
			e.mock.ExpectQuery(`INSERT INTO object_layout`).WithArgs(3, k.name, `["`+k.a+`","`+k.b+`"]`, `["`+k.c+`"]`, sqlmock.AnyArg(), "a@example.org").
				WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["`+k.a+`","`+k.b+`"]`, `["`+k.c+`"]`, 2, at, "a@example.org"))
			e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.org", "object_layout",
				jsonContains{`"kind":"` + k.name + `"`, `"old"`, `"new"`, `"version":2`}).WillReturnResult(sqlmock.NewResult(1, 1))
			rec := e.do(t, http.MethodPut, k.put(), "a@example.org", "admin_local",
				`{"order":["`+k.a+`","`+k.b+`"],"disabled":["`+k.c+`"]}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			v := olDecode(t, rec)
			if v.Status != "saved" || v.Version != 2 || len(v.Order) != 2 || v.Disabled[0] != k.c || v.UpdatedAt == nil || *v.UpdatedAt != "2026-11-07T10:15:30.123Z" {
				t.Fatalf("contrat : %s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "updated_by") || strings.Contains(rec.Body.String(), "a@example.org") {
				t.Fatal("la réponse ne contient pas updated_by")
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLayoutPut_resetIncrementeLaVersion(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	at := time.Date(2026, 11, 7, 10, 20, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM object_layout WHERE machine_id`).WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["salon"]`, `["wc2"]`, 3, at, "x"))
	e.mock.ExpectQuery(`INSERT INTO object_layout`).WithArgs(3, "lights", `[]`, `[]`, sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(olCols).AddRow(`[]`, `[]`, 4, at, "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, olKinds[0].put(), "a@example.org", "admin_local", `{"reset":true}`)
	v := olDecode(t, rec)
	if rec.Code != http.StatusOK || v.Version != 4 || v.Status != "saved" || len(v.Order) != 0 || len(v.Disabled) != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"order":[]`) || !strings.Contains(rec.Body.String(), `"disabled":[]`) {
		t.Fatalf("listes vides explicites attendues : %s", rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutPut_toutDesactiveAccepte(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	var ids []string
	for _, id := range snIDs() {
		ids = append(ids, `"`+id+`"`)
	}
	all := "[" + strings.Join(ids, ",") + "]"
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols))
	e.mock.ExpectQuery(`INSERT INTO object_layout`).WithArgs(3, "shutters", `[]`, all, sqlmock.AnyArg(), "a@example.org").
		WillReturnRows(sqlmock.NewRows(olCols).AddRow(`[]`, all, 1, time.Now(), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.do(t, http.MethodPut, olKinds[1].put(), "a@example.org", "admin_local", `{"order":[],"disabled":`+all+`}`)
	if rec.Code != http.StatusOK || len(olDecode(t, rec).Disabled) != 16 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestLayoutPut_rolesEtArmoire(t *testing.T) {
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
	for _, k := range olKinds {
		for _, c := range cases {
			t.Run(k.name+"/"+c.name, func(t *testing.T) {
				e := newLTEnv(t)
				email := "x@example.org"
				e.userRows(email, c.role, c.machine)
				if c.role != "user" && c.role != "guest_local" {
					e.userRows(email, c.role, c.machine)
					e.profileRows(email, c.role, c.machine, true)
					if c.wantExe {
						e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols))
						e.mock.ExpectQuery(`INSERT INTO object_layout`).WithArgs(c.machine.(int), k.name, `["`+k.a+`"]`, `[]`, sqlmock.AnyArg(), email).
							WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["`+k.a+`"]`, `[]`, 1, time.Now(), email))
						e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
					}
				}
				rec := e.do(t, http.MethodPut, k.put(), email, c.role, `{"order":["`+k.a+`"],"disabled":[]}`)
				if rec.Code != c.want {
					t.Fatalf("status %d, attendu %d : %s", rec.Code, c.want, rec.Body.String())
				}
				if err := e.mock.ExpectationsWereMet(); err != nil {
					t.Fatalf("écriture manquante ou inattendue : %v", err)
				}
			})
		}
	}
}

func TestLayoutPut_validation400SansEcriture(t *testing.T) {
	big := `{"order":[],"disabled":[],"x":"` + strings.Repeat("a", 16*1024) + `"}`
	for _, k := range olKinds {
		cases := []struct{ name, body, want string }{
			{"id inconnu dans order", `{"order":["nope"],"disabled":[]}`, k.unknown},
			{"id inconnu dans disabled", `{"order":[],"disabled":["nope"]}`, k.unknown},
			{"id de l'autre genre", `{"order":["` + k.other + `"],"disabled":[]}`, k.unknown},
			{"cle d'echange", `{"order":["556"],"disabled":[]}`, k.unknown},
			{"doublon dans order", `{"order":["` + k.a + `","` + k.a + `"],"disabled":[]}`, "invalid_layout"},
			{"doublon dans disabled", `{"order":[],"disabled":["` + k.a + `","` + k.a + `"]}`, "invalid_layout"},
			{"cle order absente", `{"disabled":[]}`, "invalid_layout"},
			{"cle disabled absente", `{"order":[]}`, "invalid_layout"},
			{"order null", `{"order":null,"disabled":[]}`, "invalid_layout"},
			{"order n'est pas une liste", `{"order":"salon","disabled":[]}`, "invalid_layout"},
			{"liste de nombres", `{"order":[1,2],"disabled":[]}`, "invalid_layout"},
			{"corps illisible", `nope`, "invalid_layout"},
			{"corps vide", ``, "invalid_layout"},
			{"reset faux sans listes", `{"reset":false}`, "invalid_layout"},
			{"plus de 16 Ko", big, "invalid_layout"},
		}
		for _, c := range cases {
			t.Run(k.name+"/"+c.name, func(t *testing.T) {
				e := newLTEnv(t)
				e.adminWriteSetup("a@example.org")
				rec := e.do(t, http.MethodPut, k.put(), "a@example.org", "admin_local", c.body)
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"`+c.want+`"`) {
					t.Fatalf("status %d body %s, attendu 400 %s", rec.Code, rec.Body.String(), c.want)
				}
				if err := e.mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestLayoutPut_dryRunSansEcritureNiVersion(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	at := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	// Une lecture seule est permise (version courante) ; aucune écriture, aucun audit.
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols).AddRow(`[]`, `[]`, 7, at, "x"))
	rec := e.do(t, http.MethodPut, olKinds[0].put(), "a@example.org", "admin_local", `{"order":["salon"],"disabled":["wc2"]}`, "X-Essensys-Test-Mode", "dry-run")
	v := olDecode(t, rec)
	if rec.Code != http.StatusOK || v.Status != "dry_run" || v.Version != 7 || len(v.Order) != 1 || v.Disabled[0] != "wc2" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("le dry-run ne doit rien écrire : %v", err)
	}
}

func TestLayoutPut_dryRunInvalideRefuse(t *testing.T) {
	e := newLTEnv(t)
	e.adminWriteSetup("a@example.org")
	rec := e.do(t, http.MethodPut, olKinds[0].put(), "a@example.org", "admin_local", `{"order":["nope"],"disabled":[]}`, "X-Essensys-Test-Mode", "dry-run")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestLayout_sansJetonRefuse(t *testing.T) {
	e := newLTEnv(t)
	for _, k := range olKinds {
		for _, c := range []struct{ m, p string }{{"GET", k.get()}, {"PUT", k.put()}} {
			rec := httptest.NewRecorder()
			e.handler.ServeHTTP(rec, httptest.NewRequest(c.m, c.p, strings.NewReader(`{"reset":true}`)))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s : %d, attendu 401", c.m, c.p, rec.Code)
			}
		}
	}
}

// --- synchronisation gateway ---------------------------------------------------

func TestLayoutSync_cloudPlusRecentGagne(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	cloudAt := time.Date(2026, 11, 7, 10, 15, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM object_layout`).WithArgs(3, "lights").WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["salon"]`, `["wc2"]`, 2, cloudAt, "a@example.org"))
	rec := e.olSync(olKinds[0], `{"order":["entree"],"disabled":[],"version":9,"updated_at":"2026-11-07T10:10:00.000Z","updated_by":"admin-local"}`, true)
	v := olDecode(t, rec)
	if rec.Code != http.StatusOK || v.Version != 2 || v.Order[0] != "salon" || v.UpdatedBy == nil || *v.UpdatedBy != "a@example.org" {
		t.Fatalf("le cloud doit gagner, avec updated_by : %d %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("le cloud ne doit pas être modifié : %v", err)
	}
}

func TestLayoutSync_gatewayPlusRecenteGagneEtAudit(t *testing.T) {
	for _, k := range olKinds {
		t.Run(k.name, func(t *testing.T) {
			e := newLTEnv(t)
			e.gatewaySetup(3)
			e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols).AddRow(`[]`, `[]`, 1, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
			gwAt := time.Date(2026, 11, 7, 10, 20, 0, 0, time.UTC)
			e.mock.ExpectExec(`INSERT INTO object_layout`).WithArgs(3, k.name, `["`+k.a+`"]`, `["`+k.c+`"]`, 2, gwAt, "admin-local").WillReturnResult(sqlmock.NewResult(1, 1))
			e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("gateway:gw-1", "object_layout", jsonContains{`"source":"gateway_sync"`, `"kind":"` + k.name + `"`}).WillReturnResult(sqlmock.NewResult(1, 1))
			rec := e.olSync(k, `{"order":["`+k.a+`"],"disabled":["`+k.c+`"],"version":2,"updated_at":"2026-11-07T10:20:00.000Z","updated_by":"admin-local"}`, true)
			v := olDecode(t, rec)
			if rec.Code != http.StatusOK || v.Version != 2 || v.Order[0] != k.a || v.Disabled[0] != k.c {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLayoutSync_egalitePaffaiteLeCloudReste(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	at := time.Date(2026, 11, 7, 10, 15, 0, 0, time.UTC)
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["salon"]`, `[]`, 2, at, "a@example.org"))
	rec := e.olSync(olKinds[0], `{"order":["entree"],"disabled":["wc2"],"version":2,"updated_at":"2026-11-07T10:15:00.000Z","updated_by":"a@example.org"}`, true)
	if v := olDecode(t, rec); v.Order[0] != "salon" || len(v.Disabled) != 0 {
		t.Fatalf("égalité : le cloud reste : %s", rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutSync_premierContactEnvoieSonDocument(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols))
	e.mock.ExpectExec(`INSERT INTO object_layout`).WithArgs(3, "lights", `["salon"]`, `[]`, 1, sqlmock.AnyArg(), "admin-local").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.olSync(olKinds[0], `{"order":["salon"],"disabled":[],"version":1,"updated_at":"2026-11-07T10:20:00.000Z","updated_by":"admin-local"}`, true)
	if v := olDecode(t, rec); v.Version != 1 || v.Order[0] != "salon" {
		t.Fatalf("%s", rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutSync_remiseAZeroSePropage(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["salon"]`, `["wc2"]`, 1, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
	e.mock.ExpectExec(`INSERT INTO object_layout`).WithArgs(3, "lights", `[]`, `[]`, 2, sqlmock.AnyArg(), "admin-local").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	rec := e.olSync(olKinds[0], `{"order":[],"disabled":[],"version":2,"updated_at":"2026-11-07T10:30:00.000Z","updated_by":"admin-local"}`, true)
	if v := olDecode(t, rec); v.Version != 2 || len(v.Order) != 0 || len(v.Disabled) != 0 {
		t.Fatalf("le retour à l'ordre d'origine doit se propager : %s", rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutSync_horlogeDansLeFuturRamenee(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols))
	e.mock.ExpectExec(`INSERT INTO object_layout`).WithArgs(3, "lights", `["salon"]`, `[]`, 1, sqlmock.AnyArg(), "admin-local").WillReturnResult(sqlmock.NewResult(1, 1))
	e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WillReturnResult(sqlmock.NewResult(1, 1))
	future := time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	rec := e.olSync(olKinds[0], `{"order":["salon"],"disabled":[],"version":1,"updated_at":"`+future+`","updated_by":"admin-local"}`, true)
	v := olDecode(t, rec)
	if rec.Code != http.StatusOK || v.UpdatedAt == nil || *v.UpdatedAt >= future {
		t.Fatalf("horodatage futur à ramener : %s", rec.Body.String())
	}
	if at, err := time.Parse(time.RFC3339Nano, *v.UpdatedAt); err != nil || time.Since(at) > 5*time.Second || time.Until(at) > time.Second {
		t.Fatalf("horodatage ramené à maintenant : %v (%v)", *v.UpdatedAt, err)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutSync_documentsIgnoresSansBloquer(t *testing.T) {
	cases := map[string]string{
		"version 0":                 `{"order":["salon"],"disabled":[],"version":0,"updated_at":null,"updated_by":""}`,
		"identifiant inconnu":       `{"order":["nope"],"disabled":[],"version":3,"updated_at":"2026-11-07T10:20:00.000Z","updated_by":"x"}`,
		"id de l'autre genre":       `{"order":["volet1salon"],"disabled":[],"version":3,"updated_at":"2026-11-07T10:20:00.000Z","updated_by":"x"}`,
		"doublon":                   `{"order":["salon","salon"],"disabled":[],"version":3,"updated_at":"2026-11-07T10:20:00.000Z","updated_by":"x"}`,
		"date illisible":            `{"order":["salon"],"disabled":[],"version":3,"updated_at":"pas une date","updated_by":"x"}`,
		"order n'est pas une liste": `{"order":"salon","disabled":[],"version":3,"updated_at":"2026-11-07T10:20:00.000Z","updated_by":"x"}`,
		"objet vide":                `{}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			e := newLTEnv(t)
			e.gatewaySetup(3)
			e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["entree"]`, `[]`, 1, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), "a@example.org"))
			rec := e.olSync(olKinds[0], body, true)
			if rec.Code != http.StatusOK || olDecode(t, rec).Order[0] != "entree" {
				t.Fatalf("le cloud répond son état : %d %s", rec.Code, rec.Body.String())
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("aucune écriture attendue : %v", err)
			}
		})
	}
}

func TestLayoutSync_version0NEffaceJamaisLeCloud(t *testing.T) {
	e := newLTEnv(t)
	e.gatewaySetup(3)
	e.mock.ExpectQuery(`FROM object_layout`).WillReturnRows(sqlmock.NewRows(olCols))
	rec := e.olSync(olKinds[1], `{"order":[],"disabled":[],"version":0,"updated_at":null,"updated_by":""}`, true)
	v := olDecode(t, rec)
	if rec.Code != http.StatusOK || v.Version != 0 || v.UpdatedAt != nil || len(v.Order) != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"updated_by":""`) {
		t.Fatalf("la synchro rend updated_by (vide pour le défaut) : %s", rec.Body.String())
	}
}

func TestLayoutSync_corpsInvalideEtTropLong400(t *testing.T) {
	for _, body := range []string{`nope`, ``, `[]`, `{"order":[],"x":"` + strings.Repeat("a", 64*1024) + `"}`} {
		e := newLTEnv(t)
		e.gatewaySetup(3)
		rec := e.olSync(olKinds[0], body, true)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid_body"`) {
			t.Fatalf("status %d, attendu 400 invalid_body : %.80s", rec.Code, rec.Body.String())
		}
	}
}

func TestLayoutSync_sansJeton401(t *testing.T) {
	for _, k := range olKinds {
		e := newLTEnv(t)
		rec := e.olSync(k, `{"order":["`+k.a+`"],"disabled":[],"version":99,"updated_at":"2999-01-01T00:00:00.000Z"}`, false)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s : status %d, attendu 401", k.name, rec.Code)
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

// NR: NR-backend-17 essensys-hub/essensys-user-portal-backend#59 (ticket parent essensys-feature-lifecycle#41, noms de #41 et #52 inchangés)
// Non-régression : désactiver et réordonner les lumières puis les volets ne
// change ni GET /portal/lights/names ni GET /portal/shutters/names, ni les
// tables de noms. La base simulée n'accepte que des requêtes sur object_layout
// pour les écritures (une requête sur light_names ou shutter_names ferait
// échouer le test) ; les deux réponses de noms restent celles de #41 et #52.
func TestNR_backend_17_disposition_ne_change_pas_les_noms(t *testing.T) {
	at := time.Date(2026, 11, 7, 10, 15, 30, 123000000, time.UTC)
	for _, k := range olKinds {
		e := newLTEnv(t)
		e.adminWriteSetup("a@example.org")
		e.mock.ExpectQuery(`FROM object_layout WHERE machine_id`).WillReturnRows(sqlmock.NewRows(olCols))
		e.mock.ExpectQuery(`INSERT INTO object_layout`).WithArgs(3, k.name, `["`+k.b+`","`+k.a+`"]`, `["`+k.c+`"]`, sqlmock.AnyArg(), "a@example.org").
			WillReturnRows(sqlmock.NewRows(olCols).AddRow(`["`+k.b+`","`+k.a+`"]`, `["`+k.c+`"]`, 1, at, "a@example.org"))
		e.mock.ExpectExec(`INSERT INTO portal_audit_log`).WithArgs("a@example.org", "object_layout", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
		if rec := e.do(t, http.MethodPut, k.put(), "a@example.org", "admin_local", `{"order":["`+k.b+`","`+k.a+`"],"disabled":["`+k.c+`"]}`); rec.Code != http.StatusOK {
			t.Fatalf("%s : status %d: %s", k.name, rec.Code, rec.Body.String())
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s : la disposition ne doit toucher que object_layout : %v", k.name, err)
		}
	}

	// Les noms de #41 (lumières) et de #52 (volets) : mêmes réponses, 33 et 16
	// éléments, noms personnalisés conservés, aucun champ de disposition.
	for _, c := range []struct {
		path, key, table, idCol, id, name string
		n                                 int
	}{
		{"/api/portal/lights/names", "lights", "light_names", "light_id", "dressing", "WC 3", 33},
		{"/api/portal/shutters/names", "shutters", "shutter_names", "shutter_id", "volet1salon", "Volet baie vitrée", 16},
	} {
		e := newLTEnv(t)
		e.userRows("u@example.org", "user", 3)
		e.profileRows("u@example.org", "user", 3, true)
		e.mock.ExpectQuery(`FROM ` + c.table + ` WHERE machine_id = \$1`).WithArgs(3).
			WillReturnRows(sqlmock.NewRows([]string{c.idCol, "name", "version", "updated_at", "updated_by"}).AddRow(c.id, c.name, 2, at, "a@example.org"))
		rec := e.do(t, http.MethodGet, c.path, "u@example.org", "user", "")
		var out map[string][]json.RawMessage
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out[c.key]) != c.n ||
			!strings.Contains(rec.Body.String(), `"`+c.name+`"`) ||
			strings.Contains(rec.Body.String(), "disabled") || strings.Contains(rec.Body.String(), "order") {
			t.Fatalf("%s a changé : %d %s", c.path, rec.Code, rec.Body.String())
		}
		if err := e.mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
