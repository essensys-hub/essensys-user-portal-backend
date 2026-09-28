package turnstile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerifySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("secret") != "test-secret" {
			t.Fatalf("unexpected secret")
		}
		if r.Form.Get("response") != "tok" {
			t.Fatalf("unexpected response token")
		}
		if r.Form.Get("remoteip") != "1.2.3.4" {
			t.Fatalf("unexpected remoteip")
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := NewClient("test-secret")
	c.VerifyURL = srv.URL
	c.HTTPClient = srv.Client()

	if err := c.Verify(context.Background(), "tok", "1.2.3.4"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestVerifyFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	defer srv.Close()

	c := NewClient("test-secret")
	c.VerifyURL = srv.URL
	c.HTTPClient = srv.Client()

	err := c.Verify(context.Background(), "bad", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid-input-response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := NewClient("test-secret")
	c.VerifyURL = srv.URL
	c.HTTPClient = &http.Client{Timeout: 50 * time.Millisecond}

	err := c.Verify(context.Background(), "tok", "")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestVerifyMissingTokenOrSecret(t *testing.T) {
	c := NewClient("")
	if err := c.Verify(context.Background(), "tok", ""); err == nil {
		t.Fatal("expected error for empty secret")
	}
	c = NewClient("secret")
	if err := c.Verify(context.Background(), "", ""); err == nil {
		t.Fatal("expected error for empty token")
	}
}
