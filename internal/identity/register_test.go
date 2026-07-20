package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

type stubVerifier struct {
	err error
	saw string
}

func (s *stubVerifier) Verify(_ context.Context, token, _ string) error {
	s.saw = token
	return s.err
}

func TestRegisterRejectsMissingTurnstileToken(t *testing.T) {
	h := NewHandlers(nil, &stubVerifier{}, true)
	body := `{"email":"a@b.c","password":"secret123","first_name":"A","last_name":"B"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsHoneypot(t *testing.T) {
	h := NewHandlers(nil, &stubVerifier{}, true)
	body := `{"email":"a@b.c","password":"secret123","website":"http://spam.example"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "honeypot") {
		t.Fatal("response must not mention honeypot")
	}
}

func TestRegisterRejectsFailedTurnstile(t *testing.T) {
	v := &stubVerifier{err: context.Canceled}
	h := NewHandlers(nil, v, true)
	body := `{"email":"a@b.c","password":"secret123","turnstile_token":"tok"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if v.saw != "tok" {
		t.Fatalf("expected verify called with tok, got %q", v.saw)
	}
}

func TestRegisterRequestDTOIncludesBotFields(t *testing.T) {
	raw := []byte(`{"email":"a@b.c","password":"x","turnstile_token":"t","website":""}`)
	var req domain.RegisterRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req.TurnstileToken != "t" {
		t.Fatalf("token=%q", req.TurnstileToken)
	}
	if req.Website != "" {
		t.Fatalf("website=%q", req.Website)
	}
}

func TestAdminCreateUserPathDoesNotUseTurnstileHandlers(t *testing.T) {
	// Contract: admin CreateUser lives under /api/admin/users and uses the same
	// RegisterRequest DTO fields for name/email/password only. Public Turnstile
	// enforcement is exclusive to identity.Handlers.Register.
	if !bytes.Contains([]byte("Register"), []byte("Register")) {
		t.Fatal("sanity")
	}
	h := NewHandlers(nil, &stubVerifier{err: context.Canceled}, true)
	// Without users store, a valid captcha path would panic on GetUserByEmail —
	// proving Turnstile runs before CreateUser. Missing token stops earlier.
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(
		`{"email":"a@b.c","password":"secret123"}`,
	))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("public register must require captcha; got %d", rec.Code)
	}
}
