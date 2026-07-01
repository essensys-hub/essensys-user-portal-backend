package legacyiot

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMyActionsReturnsEmptyActionsList(t *testing.T) {
	h := NewHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/myactions", nil)
	rec := httptest.NewRecorder()
	h.MyActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != `{"_de67f":null,"actions":[]}`+"\n" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestServerInfos(t *testing.T) {
	h := NewHandlers(nil, nil)
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/serverinfos", nil)
		rec := httptest.NewRecorder()
		h.ServerInfos(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("poll %d: expected 200, got %d", i, rec.Code)
		}
	}
	r := NewInfoRotator()
	for i := 0; i < 4; i++ {
		if len(r.Next()) > 30 {
			t.Fatalf("poll %d exceeds firmware limit", i)
		}
	}
}
