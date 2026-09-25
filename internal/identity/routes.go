package identity

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/turnstile"
	"github.com/go-chi/chi/v5"
)

func Mount(r chi.Router, users *data.UserStore, cfg config.Config, opts ...Option) {
	if users == nil {
		return
	}

	var verifier turnstile.Verifier
	if strings.TrimSpace(cfg.TurnstileSecretKey) != "" {
		verifier = turnstile.NewClient(cfg.TurnstileSecretKey)
	}
	h := NewHandlers(users, verifier, cfg.TurnstileEnforced(), opts...)

	limit := cfg.RegisterRateLimit
	if limit <= 0 {
		limit = 5
	}
	window := cfg.RegisterRateWindow
	if window <= 0 {
		window = time.Hour
	}
	registerLimiter := middleware.NewRateLimiter(limit, window)

	resetLimiter := middleware.NewRateLimiter(10, time.Hour)
	// Tighter than the consume limit: asking for a link mails a third party,
	// whereas consuming one only ever affects the holder of the token.
	forgotLimiter := middleware.NewRateLimiter(5, time.Hour)
	// Same shape as resetLimiter: this route only ever affects the caller's
	// own account, so it gets the same generous ceiling.
	changeLimiter := middleware.NewRateLimiter(10, time.Hour)

	r.With(registerRateLimitMiddleware(registerLimiter)).Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
	r.With(resetRateLimitMiddleware(forgotLimiter)).Post("/auth/password/forgot", h.ForgotPassword)
	r.Get("/auth/password/reset/validate", h.ValidateResetToken)
	r.With(resetRateLimitMiddleware(resetLimiter)).Post("/auth/password/reset", h.ResetPassword)
	r.Post("/auth/logout", h.Logout)
	r.Get("/auth/google/login", h.GoogleLogin)
	r.Get("/auth/google/callback", h.GoogleCallback)
	r.Get("/auth/apple/login", h.AppleLogin)
	r.Post("/auth/apple/callback", h.AppleCallback)

	r.Group(func(r chi.Router) {
		r.Use(middleware.UserJWTWithStore(users))
		r.Get("/profile", h.GetProfile)
		r.Put("/profile", h.UpdateProfile)
		r.Delete("/profile", h.DeleteProfile)
		r.Put("/profile/links", h.UpdateProfileLinks)
		r.Get("/devices/nearby", h.NearbyDevices)
	})

	// The one deliberate exemption from the password-change-required lock
	// (D3): UserJWTAllowPasswordChange rather than UserJWTWithStore, so an
	// account carrying a temporary password can reach this route and no
	// other. Rate-limited before the JWT/DB work runs, same reasoning as the
	// public routes above.
	r.With(changePasswordRateLimitMiddleware(changeLimiter), middleware.UserJWTAllowPasswordChange(users)).
		Post("/auth/password/change", h.ChangePassword)
}

func registerRateLimitMiddleware(rl *middleware.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := middleware.ClientIP(r)
			if !rl.Allow(ip) {
				log.Printf("audit action=REGISTER_BLOCKED_RATELIMIT ip=%s", ip)
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"message": "Too many registration attempts. Please try again later.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resetRateLimitMiddleware(rl *middleware.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := middleware.ClientIP(r)
			if !rl.Allow(ip) {
				log.Printf("audit action=PASSWORD_RESET_BLOCKED_RATELIMIT ip=%s", ip)
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"message": "Trop de tentatives. Merci de réessayer plus tard.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func changePasswordRateLimitMiddleware(rl *middleware.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := middleware.ClientIP(r)
			if !rl.Allow(ip) {
				log.Printf("audit action=PASSWORD_CHANGE_BLOCKED_RATELIMIT ip=%s", ip)
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"message": "Trop de tentatives. Merci de réessayer plus tard.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
