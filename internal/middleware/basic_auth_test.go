package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

type mockLegacyStore struct {
	machines map[string]*domain.LegacyMachine
}

func (m *mockLegacyStore) GetMachineByHashedPkey(hashedPkey string) (*domain.LegacyMachine, error) {
	if machine, ok := m.machines[hashedPkey]; ok {
		return machine, nil
	}
	return nil, nil
}

func (m *mockLegacyStore) RegisterUnknownMachine(hashedPkey string) (*domain.LegacyMachine, error) {
	machine := &domain.LegacyMachine{HashedPkey: hashedPkey, NoSerie: "TEST-001", IsActive: true}
	m.machines[hashedPkey] = machine
	return machine, nil
}

func (m *mockLegacyStore) UpdateMachineStatus(hashedPkey, ip, rawAuth, rawDecoded string) {}

func TestBasicAuthStrictRejectsMissingHeader(t *testing.T) {
	store := &mockLegacyStore{machines: map[string]*domain.LegacyMachine{}}
	h := BasicAuth(store, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/mystatus", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestBasicAuthLaxAllowsAnonymous(t *testing.T) {
	store := &mockLegacyStore{machines: map[string]*domain.LegacyMachine{}}
	var clientID string
	h := BasicAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID, _ = r.Context().Value(LegacyClientIDKey).(string)
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/serverinfos", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || clientID != "anonymous" {
		t.Fatalf("expected anonymous lax auth, got %d clientID=%q", rec.Code, clientID)
	}
}

func TestBasicAuthLaxInactiveMachineKeepsIdentity(t *testing.T) {
	const hashed = "c1f0d332b45731ce392f22da245daeb6"
	store := &mockLegacyStore{machines: map[string]*domain.LegacyMachine{
		hashed: {HashedPkey: hashed, NoSerie: "UNKNOWN-c1f0d332", IsActive: false},
	}}
	var clientID, hashedPkey string
	h := BasicAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID, _ = r.Context().Value(LegacyClientIDKey).(string)
		hashedPkey, _ = r.Context().Value(LegacyHashedPkeyKey).(string)
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/mystatus", nil)
	req.SetBasicAuth("c1f0d332", "b45731ce392f22da245daeb6")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for inactive lax telemetry, got %d", rec.Code)
	}
	if clientID != "UNKNOWN-c1f0d332" {
		t.Fatalf("expected UNKNOWN client id, got %q", clientID)
	}
	if hashedPkey != hashed {
		t.Fatalf("expected hashed pkey in context, got %q", hashedPkey)
	}
}

func TestBasicAuthStrictRejectsInactiveMachine(t *testing.T) {
	const hashed = "c1f0d332b45731ce392f22da245daeb6"
	store := &mockLegacyStore{machines: map[string]*domain.LegacyMachine{
		hashed: {HashedPkey: hashed, NoSerie: "UNKNOWN-c1f0d332", IsActive: false},
	}}
	h := BasicAuth(store, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/myactions", nil)
	req.SetBasicAuth("c1f0d332", "b45731ce392f22da245daeb6")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for inactive strict route, got %d", rec.Code)
	}
}
