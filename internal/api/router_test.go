package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
)

func TestHealthWithoutNewRelic(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")

	cfg := config.Config{ExchangeStaleTTL: 120 * time.Second, CORSOrigin: "https://mon.essensys.fr"}
	handler := NewRouter(nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/portal/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestConsolidatedModeScaffold(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")

	cfg := config.Config{
		ConsolidatedMode: true,
		ExchangeStaleTTL: 120 * time.Second,
		CORSOrigin:       "https://mon.essensys.fr",
	}
	handler := NewRouter(nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/portal/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("consolidated scaffold: expected 200, got %d", rec.Code)
	}
}

func TestGatewayExchangeRouteRegistered(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")

	cfg := config.Config{ExchangeStaleTTL: 120 * time.Second, CORSOrigin: "https://mon.essensys.fr"}
	handler := NewRouter(nil, nil, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/exchange", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	// Without auth middleware credentials, GatewayAuth returns 401 — route exists.
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without gateway auth, got %d", rec.Code)
	}
}

func TestConsolidatedModeAdminLoginRoute(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")
	t.Setenv("ADMIN_TOKEN", "secret-token")

	cfg := config.Config{
		ConsolidatedMode: true,
		ExchangeStaleTTL: 120 * time.Second,
		CORSOrigin:       "https://mon.essensys.fr",
	}
	handler := NewRouter(nil, &data.UserStore{}, nil, nil, nil, nil, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader([]byte(`{"token":"bad"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("consolidated admin login: expected 401, got %d", rec.Code)
	}
}
