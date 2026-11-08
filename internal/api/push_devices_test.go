package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// sonnette-plages-push-2026-11-008 (#61): push device routes, through the real
// router with a mocked database. No token ever appears in a response or a log.

const (
	pdPath  = "/api/portal/push/devices"
	pdPrefs = "/api/portal/doorbell/preferences"
	// Fake token used only by these tests.
	pdToken = "tok-secret-aaaaaaaaaaaaaaaa"
)

var pdCols = []string{"id", "user_id", "platform", "token", "app_version", "doorbell_notify", "last_seen"}

// pushUser queues the two lookups every authenticated portal call makes: the
// session check (SELECT * FROM users) and the profile read.
func (e *ltEnv) pushUser(id int, email string, gateway any) {
	e.mock.ExpectQuery(`SELECT \* FROM users WHERE email = \$1`).WithArgs(email).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
			id, email, "$2a$10$hash", "user", "A", "B", "email", "", time.Now(), time.Now(), nil,
			nil, gateway, nil, nil, nil, nil))
	e.mock.ExpectQuery(`SELECT id, email, role, first_name, last_name`).WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "role", "first_name", "last_name", "linked_machine_id", "linked_gateway_id", "linked_armoire_id"}).
			AddRow(id, email, "user", "A", "B", nil, gateway, nil))
}

func TestPushDevicePut_enregistreSansRenvoyerLeJeton(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", nil)
	e.mock.ExpectQuery(`INSERT INTO push_devices`).
		WithArgs(7, "android", pdToken, "2.3.0", true).
		WillReturnRows(sqlmock.NewRows(pdCols).AddRow(12, 7, "android", pdToken, "2.3.0", true, time.Now()))
	rec := e.do(t, http.MethodPut, pdPath, "a@example.org", "user",
		`{"platform":"android","token":"`+pdToken+`","app_version":"2.3.0","doorbell_notify":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["id"] != float64(12) || out["platform"] != "android" || out["doorbell_notify"] != true || len(out) != 3 {
		t.Fatalf("response: %v", out)
	}
	if strings.Contains(rec.Body.String(), pdToken) {
		t.Fatal("the token must not be returned")
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPushDevicePut_defautNotifyFaux(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", nil)
	e.mock.ExpectQuery(`INSERT INTO push_devices`).
		WithArgs(7, "ios", pdToken, "", false).
		WillReturnRows(sqlmock.NewRows(pdCols).AddRow(3, 7, "ios", pdToken, "", false, time.Now()))
	rec := e.do(t, http.MethodPut, pdPath, "a@example.org", "user", `{"platform":"ios","token":"`+pdToken+`"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"doorbell_notify":false`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

// The unique (platform, token) key moves a known token to the caller: the
// statement is an upsert that rewrites user_id.
func TestPushDevicePut_jetonRepris(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(8, "b@example.org", nil)
	e.mock.ExpectQuery(`ON CONFLICT \(platform, token\) DO UPDATE\s+SET user_id = EXCLUDED.user_id`).
		WithArgs(8, "android", pdToken, "2.3.0", false).
		WillReturnRows(sqlmock.NewRows(pdCols).AddRow(12, 8, "android", pdToken, "2.3.0", false, time.Now()))
	rec := e.do(t, http.MethodPut, pdPath, "b@example.org", "user",
		`{"platform":"android","token":"`+pdToken+`","app_version":"2.3.0"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPushDevicePut_erreursDeValidation(t *testing.T) {
	long := strings.Repeat("x", 4097)
	cases := []struct{ name, body, want string }{
		{"plateforme inconnue", `{"platform":"windows","token":"abc"}`, "invalid_platform"},
		{"plateforme absente", `{"token":"abc"}`, "invalid_platform"},
		{"jeton vide", `{"platform":"ios","token":"  "}`, "invalid_token"},
		{"jeton trop long", `{"platform":"ios","token":"` + long + `"}`, "invalid_token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newLTEnv(t)
			e.pushUser(7, "a@example.org", nil)
			rec := e.do(t, http.MethodPut, pdPath, "a@example.org", "user", c.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), c.want) {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("nothing must be written: %v", err)
			}
		})
	}
}

func TestPushDevicePut_sansAuthentification401(t *testing.T) {
	e := newLTEnv(t)
	rec := e.doAnon(http.MethodPut, pdPath, `{"platform":"ios","token":"abc"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPushDeviceDelete_proprietaire204(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", nil)
	e.mock.ExpectExec(`DELETE FROM push_devices WHERE id = \$1 AND user_id = \$2`).WithArgs(12, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	rec := e.do(t, http.MethodDelete, pdPath+"/12", "a@example.org", "user", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPushDeviceDelete_appareilDAutrui404(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", nil)
	e.mock.ExpectExec(`DELETE FROM push_devices WHERE id = \$1 AND user_id = \$2`).WithArgs(99, 7).
		WillReturnResult(sqlmock.NewResult(0, 0))
	rec := e.do(t, http.MethodDelete, pdPath+"/99", "a@example.org", "user", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPushDeviceDelete_identifiantIllisible404(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", nil)
	rec := e.do(t, http.MethodDelete, pdPath+"/abc", "a@example.org", "user", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestDoorbellPreferences_sansGatewayEtSansJeton(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", nil)
	e.mock.ExpectQuery(`FROM push_devices WHERE user_id = \$1`).WithArgs(7).
		WillReturnRows(sqlmock.NewRows(pdCols).
			AddRow(12, 7, "android", pdToken, "2.3.0", true, time.Now()).
			AddRow(13, 7, "ios", pdToken+"-ios", "2.3.0", false, time.Now()))
	rec := e.do(t, http.MethodGet, pdPrefs, "a@example.org", "user", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Available bool             `json:"available"`
		Devices   []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Available || len(out.Devices) != 2 || out.Devices[0]["doorbell_notify"] != true || out.Devices[1]["platform"] != "ios" {
		t.Fatalf("response: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "tok-secret") || strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatal("no token in the preferences")
	}
}

func TestDoorbellPreferences_gatewayLieeDisponible(t *testing.T) {
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", "gw-1")
	e.mock.ExpectQuery(`FROM push_devices WHERE user_id = \$1`).WithArgs(7).WillReturnRows(sqlmock.NewRows(pdCols))
	e.mock.ExpectQuery(`SELECT last_seen FROM gateway_sessions`).WithArgs("gw-1").
		WillReturnRows(sqlmock.NewRows([]string{"last_seen"}).AddRow(time.Now().Add(-time.Hour)))
	rec := e.do(t, http.MethodGet, pdPrefs, "a@example.org", "user", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":true`) || !strings.Contains(rec.Body.String(), `"devices":[]`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func (e *ltEnv) doAnon(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

// A failing store must not leak the token through the logs.
func TestPushDevicePut_aucunJetonDansLeJournal(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	e := newLTEnv(t)
	e.pushUser(7, "a@example.org", nil)
	e.mock.ExpectQuery(`INSERT INTO push_devices`).WillReturnError(errors.New("db down"))
	rec := e.do(t, http.MethodPut, pdPath, "a@example.org", "user", `{"platform":"android","token":"`+pdToken+`"}`)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), pdToken) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(buf.String(), pdToken) || !strings.Contains(buf.String(), "push device upsert") {
		t.Fatalf("log: %q", buf.String())
	}
}
