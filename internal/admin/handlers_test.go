package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminLoginInvalidToken(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "secret-token")
	h := NewHandlers(Deps{})

	body, _ := json.Marshal(map[string]string{"token": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAdminLoginValidToken(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "secret-token")
	h := NewHandlers(Deps{})

	body, _ := json.Marshal(map[string]string{"token": "secret-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSubscribeMissingEmail(t *testing.T) {
	h := NewHandlers(Deps{News: nil})
	req := httptest.NewRequest(http.MethodPost, "/api/newsletter/subscribe", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	h.Subscribe(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without store, got %d", rec.Code)
	}
}
