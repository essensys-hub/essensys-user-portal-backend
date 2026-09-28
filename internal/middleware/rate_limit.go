package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	counts map[string][]time.Time
	limit  int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		counts: make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

// Allow returns true if the key is within the sliding window limit and records the attempt.
func (rl *RateLimiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	times := rl.counts[key]
	var kept []time.Time
	for _, t := range times {
		if now.Sub(t) <= rl.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.counts[key] = kept
		return false
	}
	kept = append(kept, now)
	rl.counts[key] = kept
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Authorization")
		if key == "" {
			key = r.RemoteAddr
		}
		if !rl.Allow(key) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MiddlewareByIP rate-limits using the client IP (X-Forwarded-For / RemoteAddr).
func (rl *RateLimiter) MiddlewareByIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(ClientIP(r)) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP returns the best-effort client IP for rate limiting and audit.
func ClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return strings.TrimSpace(rip)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		return host[:i]
	}
	return host
}
