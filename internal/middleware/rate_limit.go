package middleware

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	counts   map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		counts: make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (rl *rateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Authorization")
		if key == "" {
			key = r.RemoteAddr
		}
		now := time.Now()
		rl.mu.Lock()
		times := rl.counts[key]
		var kept []time.Time
		for _, t := range times {
			if now.Sub(t) <= rl.window {
				kept = append(kept, t)
			}
		}
		if len(kept) >= rl.limit {
			rl.mu.Unlock()
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		kept = append(kept, now)
		rl.counts[key] = kept
		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
