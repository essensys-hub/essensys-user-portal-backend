package legacyiot

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMyActionsReturnsEmptyObject(t *testing.T) {
	h := NewHandlers(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/myactions", nil)
	rec := httptest.NewRecorder()
	h.MyActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "{}" {
		t.Fatalf("expected {}, got %s", rec.Body.String())
	}
}

func TestServerInfos(t *testing.T) {
	h := NewHandlers(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/serverinfos", nil)
	rec := httptest.NewRecorder()
	h.ServerInfos(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
