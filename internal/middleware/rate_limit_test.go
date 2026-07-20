package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(2, time.Hour)
	if !rl.Allow("1.1.1.1") || !rl.Allow("1.1.1.1") {
		t.Fatal("first two should allow")
	}
	if rl.Allow("1.1.1.1") {
		t.Fatal("third should deny")
	}
	if !rl.Allow("2.2.2.2") {
		t.Fatal("other IP should allow")
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "9.9.9.9, 8.8.8.8")
	if got := ClientIP(req); got != "9.9.9.9" {
		t.Fatalf("got %q", got)
	}
}
