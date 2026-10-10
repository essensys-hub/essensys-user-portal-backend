package support

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// memStore is an in-memory Store with the same semantics as the SQL one.
type memStore struct {
	mu     sync.Mutex
	rows   []data.SupportReport
	now    func() time.Time
	locked bool
}

func (m *memStore) Create(_ context.Context, r *data.SupportReport) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.ID, r.Status, r.CreatedAt, r.UpdatedAt = len(m.rows)+1, data.SupportStatusPending, m.now(), m.now()
	m.rows = append(m.rows, *r)
	return nil
}
func (m *memStore) ListByUser(_ context.Context, uid int) ([]data.SupportReport, error) {
	var out []data.SupportReport
	for i := len(m.rows) - 1; i >= 0; i-- {
		if m.rows[i].UserID == uid {
			out = append(out, m.rows[i])
		}
	}
	return out, nil
}
func (m *memStore) CountSince(_ context.Context, uid int, since time.Time) (int, time.Time, error) {
	n, oldest := 0, time.Time{}
	for _, r := range m.rows {
		if r.UserID == uid && r.CreatedAt.After(since) {
			if n == 0 || r.CreatedAt.Before(oldest) {
				oldest = r.CreatedAt
			}
			n++
		}
	}
	return n, oldest, nil
}
func (m *memStore) get(id int) *data.SupportReport { return &m.rows[id-1] }
func (m *memStore) MarkSent(_ context.Context, id int, repo string, n int, url string) error {
	r := m.get(id)
	r.Status, r.Repo, r.IssueNumber, r.IssueURL, r.Payload = data.SupportStatusOpen,
		sql.NullString{String: repo, Valid: true}, sql.NullInt64{Int64: int64(n), Valid: true}, sql.NullString{String: url, Valid: true}, []byte(`{}`)
	r.SyncedAt = sql.NullTime{Time: m.now(), Valid: true}
	return nil
}
func (m *memStore) MarkFailed(_ context.Context, id int, msg string) error {
	r := m.get(id)
	r.Attempts++
	r.LastError = sql.NullString{String: msg, Valid: true}
	return nil
}
func (m *memStore) ListPending(_ context.Context, _ int) ([]data.SupportReport, error) {
	var out []data.SupportReport
	for _, r := range m.rows {
		if r.Status == data.SupportStatusPending {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memStore) ListOpenToSync(_ context.Context, before time.Time, _ int) ([]data.SupportReport, error) {
	var out []data.SupportReport
	for _, r := range m.rows {
		if r.Status == data.SupportStatusOpen && (!r.SyncedAt.Valid || r.SyncedAt.Time.Before(before)) {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memStore) UpdateStatus(_ context.Context, id int, status string) error {
	r := m.get(id)
	r.Status, r.SyncedAt = status, sql.NullTime{Time: m.now(), Valid: true}
	return nil
}
func (m *memStore) WithSyncLock(_ context.Context, fn func() error) error {
	if m.locked {
		return data.ErrSyncLockBusy
	}
	return fn()
}

type memUsers map[string]*domain.User

func (u memUsers) GetUserByEmail(email string) (*domain.User, error) { return u[email], nil }

type harness struct {
	store *memStore
	gh    *fakeGitHub
	svc   *Service
	srv   http.Handler
	clock time.Time
}

func newHarness(t *testing.T, withGitHub bool) *harness {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-test-secret-0123456789")
	h := &harness{clock: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	now := func() time.Time { return h.clock }
	h.store = &memStore{now: now}
	var gh GitHub
	if withGitHub {
		var c *AppClient
		h.gh, c = newFakeGitHub(t)
		gh = c
	}
	h.svc = NewService(h.store, gh, Settings{PseudonymKey: []byte("pseudonym-key-pseudonym-key"),
		PublicRepo: "essensys-hub/essensys-support-site", PrivateRepo: "essensys-hub/essensys-support", ProjectID: "PVT_x"})
	h.svc.Now = now
	users := memUsers{
		"alice@example.fr": {ID: 1, Email: "alice@example.fr", Role: domain.RoleUser, FirstName: "Alice", LastName: "Martin"},
		"bob@example.fr":   {ID: 2, Email: "bob@example.fr", Role: domain.RoleUser},
	}
	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) { Mount(r, users, h.svc) })
	h.srv = r
	return h
}

func (h *harness) do(t *testing.T, method, email string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, "/api/support/reports", &buf)
	if email != "" {
		tok, err := middleware.GenerateJWT(email, domain.RoleUser, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	h.srv.ServeHTTP(rec, req)
	return rec
}

func bugInput() map[string]any {
	return map[string]any{"kind": "bug", "title": "Les volets ne répondent plus", "app": "android", "version": "2.0.0", "mode": "cloud",
		"actual": "Rien ne se passe quand j'appuie sur Fermer.", "expected": "Le volet se ferme.", "steps": "Volets > Cuisine > Fermer"}
}

func TestCreateBug_OpensAnonymisedPublicIssue(t *testing.T) {
	h := newHarness(t, true)
	rec := h.do(t, http.MethodPost, "alice@example.fr", bugInput())
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var v reportView
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Status != "open" || !strings.HasPrefix(v.Ref, "R-") {
		t.Fatalf("view %+v", v)
	}
	if len(h.gh.created) != 1 || h.gh.created[0]["_path"] != "/repos/essensys-hub/essensys-support-site/issues" {
		t.Fatalf("created %+v", h.gh.created)
	}
	body := h.gh.created[0]["body"].(string) + h.gh.created[0]["title"].(string)
	for _, leak := range []string{"alice", "Alice", "Martin", "example.fr"} {
		if strings.Contains(body, leak) {
			t.Fatalf("public issue leaks %q:\n%s", leak, body)
		}
	}
	if !strings.Contains(body, UserPseudonym(h.svc.Settings.PseudonymKey, 1)) {
		t.Fatal("pseudonym missing")
	}
	if string(h.store.get(1).Payload) != "{}" {
		t.Fatal("payload must be cleared once on GitHub")
	}
}

// NR: NR-backend-3 essensys-hub/essensys-feature-lifecycle#15
func Test_NR_backend_3_sixth_report_in_24h_is_429(t *testing.T) {
	h := newHarness(t, false)
	for i := 0; i < 5; i++ {
		if rec := h.do(t, http.MethodPost, "alice@example.fr", bugInput()); rec.Code != http.StatusCreated {
			t.Fatalf("report %d: %d", i+1, rec.Code)
		}
		h.clock = h.clock.Add(time.Hour)
	}
	rec := h.do(t, http.MethodPost, "alice@example.fr", bugInput())
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d, want 429 with Retry-After", rec.Code)
	}
	if len(h.store.rows) != 5 {
		t.Fatalf("%d rows stored, want 5", len(h.store.rows))
	}
	if rec := h.do(t, http.MethodPost, "bob@example.fr", bugInput()); rec.Code != http.StatusCreated {
		t.Fatalf("other account blocked: %d", rec.Code)
	}
	h.clock = h.clock.Add(20 * time.Hour)
	if rec := h.do(t, http.MethodPost, "alice@example.fr", bugInput()); rec.Code != http.StatusCreated {
		t.Fatalf("window should have freed a slot: %d", rec.Code)
	}
}

// NR: NR-backend-4 essensys-hub/essensys-feature-lifecycle#15
func Test_NR_backend_4_no_session_is_401(t *testing.T) {
	h := newHarness(t, true)
	if rec := h.do(t, http.MethodPost, "", bugInput()); rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST got %d", rec.Code)
	}
	if rec := h.do(t, http.MethodGet, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET got %d", rec.Code)
	}
	if rec := h.do(t, http.MethodPost, "ghost@example.fr", bugInput()); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown account got %d", rec.Code)
	}
	if len(h.gh.created) != 0 {
		t.Fatal("issue created without a session")
	}
}

// NR: NR-backend-5 essensys-hub/essensys-feature-lifecycle#15
func Test_NR_backend_5_user_only_sees_own_reports(t *testing.T) {
	h := newHarness(t, false)
	h.do(t, http.MethodPost, "alice@example.fr", bugInput())
	b := bugInput()
	b["title"] = "Rapport de Bob"
	h.do(t, http.MethodPost, "bob@example.fr", b)
	rec := h.do(t, http.MethodGet, "alice@example.fr", nil)
	if strings.Contains(rec.Body.String(), "Rapport de Bob") || !strings.Contains(rec.Body.String(), "volets") {
		t.Fatalf("alice sees: %s", rec.Body)
	}
}

// NR: NR-backend-6 essensys-hub/essensys-feature-lifecycle#15
func Test_NR_backend_6_incident_goes_to_private_repo(t *testing.T) {
	h := newHarness(t, true)
	rec := h.do(t, http.MethodPost, "alice@example.fr", map[string]any{"kind": "incident", "title": "Plus rien ne marche",
		"impact": "local", "since": "ce matin", "actual": "Aucune lumière ne répond."})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if h.gh.created[0]["_path"] != "/repos/essensys-hub/essensys-support/issues" {
		t.Fatalf("incident sent to %v", h.gh.created[0]["_path"])
	}
	list := h.do(t, http.MethodGet, "alice@example.fr", nil).Body.String()
	if strings.Contains(list, "essensys-support/") || strings.Contains(list, "issue_url") {
		t.Fatalf("private repo exposed: %s", list)
	}
}

// NR: NR-backend-7 essensys-hub/essensys-feature-lifecycle#15
func Test_NR_backend_7_github_down_keeps_pending(t *testing.T) {
	h := newHarness(t, true)
	h.gh.fail = 1
	rec := h.do(t, http.MethodPost, "alice@example.fr", bugInput())
	var v reportView
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if rec.Code != http.StatusCreated || v.Status != "pending" {
		t.Fatalf("got %d %+v", rec.Code, v)
	}
	if e := h.store.get(1).LastError.String; strings.Contains(e, "volet") {
		t.Fatalf("last_error carries user content: %q", e)
	}
	if err := h.svc.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := h.store.get(1).Status; got != "open" || len(h.gh.created) != 1 {
		t.Fatalf("after retry status=%s created=%d", got, len(h.gh.created))
	}
}

func TestCreate_SensitiveContentIs422WithoutEcho(t *testing.T) {
	h := newHarness(t, true)
	b := bugInput()
	b["actual"] = "mon mot de passe : Soleil2026"
	rec := h.do(t, http.MethodPost, "alice@example.fr", b)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	var out map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["field"] != "actual" || out["category"] != "password" || strings.Contains(rec.Body.String(), "Soleil2026") {
		t.Fatalf("body %s", rec.Body)
	}
	if len(h.store.rows) != 0 || len(h.gh.created) != 0 {
		t.Fatal("rejected report was stored or sent")
	}
}

func TestCreate_FieldValidation(t *testing.T) {
	h := newHarness(t, false)
	cases := map[string]func(map[string]any){
		"title":  func(b map[string]any) { b["title"] = "  " },
		"actual": func(b map[string]any) { b["actual"] = "" },
		"kind":   func(b map[string]any) { b["kind"] = "feature" },
		"app":    func(b map[string]any) { b["app"] = "windows" },
	}
	for field, mutate := range cases {
		b := bugInput()
		mutate(b)
		rec := h.do(t, http.MethodPost, "alice@example.fr", b)
		var out map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if rec.Code != http.StatusBadRequest || out["field"] != field {
			t.Errorf("%s: got %d %s", field, rec.Code, rec.Body)
		}
	}
	b := bugInput()
	b["actual"] = strings.Repeat("a", maxTextLen+1)
	if rec := h.do(t, http.MethodPost, "alice@example.fr", b); rec.Code != http.StatusBadRequest {
		t.Errorf("too long: got %d", rec.Code)
	}
}

func TestSyncOnce_MapsClosedIssueToResolved(t *testing.T) {
	h := newHarness(t, true)
	h.do(t, http.MethodPost, "alice@example.fr", bugInput())
	h.clock = h.clock.Add(11 * time.Minute)
	if err := h.svc.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := h.store.get(1).Status; got != "resolved" {
		t.Fatalf("status %s, want resolved", got)
	}
}

func TestSyncOnce_SkipsWhenLockBusy(t *testing.T) {
	h := newHarness(t, true)
	h.gh.fail = 1
	h.do(t, http.MethodPost, "alice@example.fr", bugInput())
	h.store.locked = true
	if err := h.svc.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.store.get(1).Status != "pending" {
		t.Fatal("ran without the lock")
	}
}

func TestStatusFromIssue(t *testing.T) {
	cases := []struct {
		iss  Issue
		want string
	}{
		{Issue{State: "open"}, "open"},
		{Issue{State: "closed", StateReason: "completed"}, "resolved"},
		{Issue{State: "closed", StateReason: "not_planned"}, "closed"},
	}
	for _, c := range cases {
		if got := StatusFromIssue(c.iss); got != c.want {
			t.Errorf("%+v: got %s", c.iss, got)
		}
	}
}
