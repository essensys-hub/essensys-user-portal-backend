package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/push"
	"github.com/jmoiron/sqlx"
)

// sonnette-plages-push-2026-11-008 (#61): POST /api/gateway/doorbell/ring,
// through the real router, a mocked database and fake FCM / APNs servers
// (httptest). No real notification, no outgoing network call.

const (
	drPath  = "/api/gateway/doorbell/ring"
	drToken = "gw-token-for-tests"
	// Fake device tokens used only by these tests.
	drAndroidToken = "android-token-for-tests-aaaaaaaa"
	drIOSToken     = "ios-token-for-tests-bbbbbbbbbbbb"
)

// fakeProvider records the requests of a fake FCM or APNs server.
type fakeProvider struct {
	mu     sync.Mutex
	paths  []string
	bodies []map[string]any
	status int
	body   string
	srv    *httptest.Server
}

func newFakeProvider(t *testing.T, tls bool, status int, body string) *fakeProvider {
	t.Helper()
	f := &fakeProvider{status: status, body: body}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.bodies = append(f.bodies, m)
		f.mu.Unlock()
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.body)
	})
	if tls {
		f.srv = httptest.NewTLSServer(h)
	} else {
		f.srv = httptest.NewServer(h)
	}
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeProvider) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.paths)
}

func fakeDispatcher(t *testing.T, fcm, apns *fakeProvider) *push.Dispatcher {
	t.Helper()
	d := &push.Dispatcher{}
	if fcm != nil {
		d.Android = push.NewFCMSenderWithClient(fcm.srv.URL, "demo-project", fcm.srv.Client())
	}
	if apns != nil {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		der, _ := x509.MarshalPKCS8PrivateKey(key)
		p8 := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
		s, err := push.NewAPNsSender(push.APNsConfig{KeyP8: p8, KeyID: "KEYID", TeamID: "TEAMID", BundleID: "essensys.essensys-iphone"})
		if err != nil {
			t.Fatal(err)
		}
		d.IOS = s.WithEndpoint(apns.srv.URL, apns.srv.Client())
	}
	return d
}

func newPushEnv(t *testing.T, d *push.Dispatcher) *ltEnv {
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
	h := NewRouter(data.NewPortalStore(sx), data.NewUserStore(sx), nil, nil, nil, nil, nil, nil, nil, cfg, WithPush(d))
	return &ltEnv{handler: h, mock: mock}
}

func (e *ltEnv) ringAuth(gateway string) {
	sum := sha256.Sum256([]byte(drToken))
	e.mock.ExpectQuery(`FROM gateway_sessions WHERE gateway_id`).WithArgs(gateway).
		WillReturnRows(sqlmock.NewRows([]string{"gateway_id", "token_hash", "machine_id", "eth0_mac", "eth1_mac", "last_seen"}).
			AddRow(gateway, hex.EncodeToString(sum[:]), 3, nil, nil, nil))
}

func (e *ltEnv) ringSeen(gateway, ring string, seen bool) {
	rows := sqlmock.NewRows([]string{"?column?"})
	if seen {
		rows.AddRow(1)
	}
	e.mock.ExpectQuery(`SELECT 1 FROM doorbell_rings WHERE gateway_id = \$1 AND ring_id = \$2`).WithArgs(gateway, ring).WillReturnRows(rows)
}

func (e *ltEnv) ringRecipients(gateway string, devices ...[]driver.Value) {
	rows := sqlmock.NewRows(pdCols)
	for _, d := range devices {
		rows.AddRow(d...)
	}
	e.mock.ExpectQuery(`u.linked_gateway_id = \$1 AND d.doorbell_notify = TRUE`).WithArgs(gateway).WillReturnRows(rows)
}

func (e *ltEnv) ringRecorded(gateway, ring string) {
	e.mock.ExpectExec(`INSERT INTO doorbell_rings`).WithArgs(gateway, ring).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mock.ExpectExec(`DELETE FROM doorbell_rings WHERE received_at`).WillReturnResult(sqlmock.NewResult(0, 0))
}

func (e *ltEnv) ring(body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, drPath, strings.NewReader(body))
	req.Header.Set("X-Gateway-ID", "gw-1")
	req.Header.Set("Authorization", "Bearer "+drToken)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func androidDev(id int) []driver.Value {
	return []driver.Value{id, 7, "android", drAndroidToken, "2.3.0", true, time.Now()}
}
func iosDev(id int) []driver.Value {
	return []driver.Value{id, 8, "ios", drIOSToken, "2.3.0", true, time.Now()}
}

func ringResp(t *testing.T, rec *httptest.ResponseRecorder) (string, int) {
	t.Helper()
	var out struct {
		Status     string `json:"status"`
		Recipients int    `json:"recipients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response %q: %v", rec.Body.String(), err)
	}
	return out.Status, out.Recipients
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

const ringBody = `{"ring_id":"ring-1","at":"2026-11-08T10:15:30.123Z"}`

func TestDoorbellRing_appuiRecuEnvoyeAuxDeuxPlateformes(t *testing.T) {
	logs := captureLog(t)
	fcm := newFakeProvider(t, false, 200, `{}`)
	apns := newFakeProvider(t, true, 200, ``)
	e := newPushEnv(t, fakeDispatcher(t, fcm, apns))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", androidDev(12), iosDev(13))
	e.ringRecorded("gw-1", "ring-1")
	rec := e.ring(ringBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if st, n := ringResp(t, rec); st != "accepted" || n != 2 {
		t.Fatalf("status %q recipients %d", st, n)
	}
	if fcm.calls() != 1 || apns.calls() != 1 {
		t.Fatalf("fcm %d apns %d calls", fcm.calls(), apns.calls())
	}
	m := fcm.bodies[0]["message"].(map[string]any)
	if m["token"] != drAndroidToken || m["data"].(map[string]any)["ring_id"] != "ring-1" ||
		m["notification"].(map[string]any)["title"] != "Quelqu'un sonne" {
		t.Fatalf("fcm body: %v", m)
	}
	if apns.paths[0] != "/3/device/"+drIOSToken || apns.bodies[0]["ring_id"] != "ring-1" {
		t.Fatalf("apns: %v %v", apns.paths, apns.bodies)
	}
	for _, s := range []string{rec.Body.String(), logs.String()} {
		if strings.Contains(s, drAndroidToken) || strings.Contains(s, drIOSToken) {
			t.Fatal("no device token in a response or a log")
		}
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDoorbellRing_idempotent(t *testing.T) {
	fcm := newFakeProvider(t, false, 200, `{}`)
	e := newPushEnv(t, fakeDispatcher(t, fcm, nil))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", true) // already recorded: nothing else is read or written
	rec := e.ring(ringBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if st, n := ringResp(t, rec); st != "accepted" || n != 0 {
		t.Fatalf("status %q recipients %d", st, n)
	}
	if fcm.calls() != 0 {
		t.Fatal("a repeated ring must not notify again")
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Recipients are chosen in SQL: only users linked to the calling gateway, only
// devices with doorbell_notify = TRUE. A gateway without such a device gets no
// notification (other gateway, preference off).
func TestDoorbellRing_aucunDestinataire(t *testing.T) {
	fcm := newFakeProvider(t, false, 200, `{}`)
	apns := newFakeProvider(t, true, 200, ``)
	e := newPushEnv(t, fakeDispatcher(t, fcm, apns))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1")
	e.ringRecorded("gw-1", "ring-1")
	rec := e.ring(ringBody)
	if st, n := ringResp(t, rec); rec.Code != 200 || st != "accepted" || n != 0 {
		t.Fatalf("%d %q %d", rec.Code, st, n)
	}
	if fcm.calls()+apns.calls() != 0 {
		t.Fatal("no provider call without recipient")
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDoorbellRing_gatewayNonAuthentifiee401(t *testing.T) {
	fcm := newFakeProvider(t, false, 200, `{}`)
	e := newPushEnv(t, fakeDispatcher(t, fcm, nil))
	e.mock.ExpectQuery(`FROM gateway_sessions WHERE gateway_id`).WithArgs("gw-1").
		WillReturnRows(sqlmock.NewRows([]string{"gateway_id", "token_hash", "machine_id", "eth0_mac", "eth1_mac", "last_seen"}).
			AddRow("gw-1", "not-the-hash", 3, nil, nil, nil))
	if rec := e.ring(ringBody); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, drPath, strings.NewReader(ringBody))
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credentials: %d", rec.Code)
	}
	if fcm.calls() != 0 {
		t.Fatal("nothing must be processed")
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDoorbellRing_unAppuiParDixSecondesEtParGateway(t *testing.T) {
	fcm := newFakeProvider(t, false, 200, `{}`)
	e := newPushEnv(t, fakeDispatcher(t, fcm, nil))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", androidDev(12))
	e.ringRecorded("gw-1", "ring-1")
	if rec := e.ring(ringBody); rec.Code != 200 {
		t.Fatalf("first ring: %d", rec.Code)
	}
	// Another ring, another id, right after: refused, nothing else is touched.
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-2", false)
	rec := e.ring(`{"ring_id":"ring-2","at":"2026-11-08T10:15:32Z"}`)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "too_many_rings") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if fcm.calls() != 1 {
		t.Fatalf("fcm calls %d, want 1", fcm.calls())
	}
	// The same ring id resent inside the window is a duplicate (200), not a 429.
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", true)
	if rec := e.ring(ringBody); rec.Code != 200 {
		t.Fatalf("duplicate inside the window: %d", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDoorbellRing_dryRunSansEnvoiNiEcriture(t *testing.T) {
	fcm := newFakeProvider(t, false, 200, `{}`)
	apns := newFakeProvider(t, true, 200, ``)
	e := newPushEnv(t, fakeDispatcher(t, fcm, apns))
	e.ringAuth("gw-1")
	rec := e.ring(ringBody, "X-Essensys-Test-Mode", "dry-run")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if st, n := ringResp(t, rec); st != "dry_run" || n != 0 {
		t.Fatalf("%q %d", st, n)
	}
	if fcm.calls()+apns.calls() != 0 {
		t.Fatal("dry-run must not call a provider")
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("dry-run must not touch the database: %v", err)
	}
}

func TestDoorbellRing_atFuturRamenéALHeureDuCloud(t *testing.T) {
	fcm := newFakeProvider(t, false, 200, `{}`)
	e := newPushEnv(t, fakeDispatcher(t, fcm, nil))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", androidDev(12))
	e.ringRecorded("gw-1", "ring-1")
	future := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano)
	rec := e.ring(`{"ring_id":"ring-1","at":"` + future + `"}`)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	at, err := time.Parse(time.RFC3339Nano, fcm.bodies[0]["message"].(map[string]any)["data"].(map[string]any)["at"].(string))
	if err != nil || time.Since(at) > time.Minute || at.After(time.Now().Add(time.Second)) {
		t.Fatalf("at %v err %v", at, err)
	}
}

func TestDoorbellRing_atProcheDuFuturEtPasseConserve(t *testing.T) {
	fcm := newFakeProvider(t, false, 200, `{}`)
	e := newPushEnv(t, fakeDispatcher(t, fcm, nil))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", androidDev(12))
	e.ringRecorded("gw-1", "ring-1")
	if rec := e.ring(ringBody); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if got := fcm.bodies[0]["message"].(map[string]any)["data"].(map[string]any)["at"]; got != "2026-11-08T10:15:30.123Z" {
		t.Fatalf("a past `at` is kept: %v", got)
	}
}

func TestDoorbellRing_corpsInvalide400(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"json cassé", `{`, "invalid_body"},
		{"ring_id absent", `{"at":"2026-11-08T10:15:30Z"}`, "invalid_ring_id"},
		{"ring_id trop long", `{"ring_id":"` + strings.Repeat("x", 129) + `","at":"2026-11-08T10:15:30Z"}`, "invalid_ring_id"},
		{"at absent", `{"ring_id":"r"}`, "invalid_at"},
		{"at illisible", `{"ring_id":"r","at":"hier"}`, "invalid_at"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newPushEnv(t, &push.Dispatcher{})
			e.ringAuth("gw-1")
			rec := e.ring(c.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), c.want) {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if err := e.mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDoorbellRing_fournisseursAbsentsUnSeulPushUnavailable(t *testing.T) {
	logs := captureLog(t)
	e := newPushEnv(t, &push.Dispatcher{})
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", androidDev(12), iosDev(13))
	e.ringRecorded("gw-1", "ring-1")
	rec := e.ring(ringBody)
	if st, n := ringResp(t, rec); rec.Code != 200 || st != "accepted" || n != 0 {
		t.Fatalf("%d %q %d", rec.Code, st, n)
	}
	if c := strings.Count(logs.String(), "push_unavailable"); c != 1 {
		t.Fatalf("push_unavailable logged %d times, want 1: %q", c, logs.String())
	}
}

func TestDoorbellRing_jetonPerimeSupprimeLAppareil(t *testing.T) {
	fcm := newFakeProvider(t, false, 404, `{"error":{"code":404,"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`)
	e := newPushEnv(t, fakeDispatcher(t, fcm, nil))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", androidDev(12))
	e.ringRecorded("gw-1", "ring-1")
	e.mock.ExpectExec(`DELETE FROM push_devices WHERE id = \$1`).WithArgs(12).WillReturnResult(sqlmock.NewResult(0, 1))
	rec := e.ring(ringBody)
	if st, n := ringResp(t, rec); rec.Code != 200 || st != "accepted" || n != 0 {
		t.Fatalf("%d %q %d", rec.Code, st, n)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("the stale device must be deleted: %v", err)
	}
}

func TestDoorbellRing_apns410SupprimeLAppareil(t *testing.T) {
	apns := newFakeProvider(t, true, 410, `{"reason":"Unregistered"}`)
	e := newPushEnv(t, fakeDispatcher(t, nil, apns))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", iosDev(13))
	e.ringRecorded("gw-1", "ring-1")
	e.mock.ExpectExec(`DELETE FROM push_devices WHERE id = \$1`).WithArgs(13).WillReturnResult(sqlmock.NewResult(0, 1))
	if rec := e.ring(ringBody); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDoorbellRing_fcmEnPanneNeBloquePasLiPhone(t *testing.T) {
	logs := captureLog(t)
	fcm := newFakeProvider(t, false, 500, `{"error":{"status":"INTERNAL"}}`)
	apns := newFakeProvider(t, true, 200, ``)
	e := newPushEnv(t, fakeDispatcher(t, fcm, apns))
	e.ringAuth("gw-1")
	e.ringSeen("gw-1", "ring-1", false)
	e.ringRecipients("gw-1", androidDev(12), iosDev(13))
	e.ringRecorded("gw-1", "ring-1")
	rec := e.ring(ringBody)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if st, n := ringResp(t, rec); st != "accepted" || n != 1 {
		t.Fatalf("%q %d", st, n)
	}
	if apns.calls() != 1 || fcm.calls() != 1 {
		t.Fatalf("fcm %d apns %d", fcm.calls(), apns.calls())
	}
	if strings.Contains(logs.String(), drAndroidToken) || !strings.Contains(logs.String(), "send failed") {
		t.Fatalf("log: %q", logs.String())
	}
	if err := e.mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
