package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminAuthStaticToken(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	called := false
	h := AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected static token auth, got %d called=%v", rec.Code, called)
	}
}

func TestAdminAuthRejectsMissingToken(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	h := AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
