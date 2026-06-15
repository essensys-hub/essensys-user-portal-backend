package portal

import (
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/handlers"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Mount registers user portal routes (/api/portal/*).
func Mount(r chi.Router, h *handlers.Handler, injectLimiter *middleware.RateLimiter) {
	r.Route("/portal", func(r chi.Router) {
		r.Get("/health", h.Health)

		r.Group(func(r chi.Router) {
			r.Use(middleware.UserJWT)
		r.Post("/link-request", h.CreateLinkRequest)
		r.Get("/link-request/status", h.LinkRequestStatus)
		r.Get("/gateway/status", h.GatewayStatus)
		r.With(injectLimiter.Middleware).Post("/inject", h.Inject)
		r.Get("/exchange", h.GetExchange)
		r.Get("/history/latest", h.GetHistoryLatest)
		r.Post("/web/actions", h.PostWebActions)

		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.AdminJWT)
			r.Get("/link-requests", h.ListPendingLinkRequests)
			r.Put("/link-requests/{id}", h.ReviewLinkRequest)
			r.Get("/gateway-sessions", h.ListGatewaySessions)
			r.Post("/gateways/register", h.RegisterGateway)
		})
		})
	})
}

// DefaultInjectLimiter matches production rate limit for inject.
func DefaultInjectLimiter() *middleware.RateLimiter {
	return middleware.NewRateLimiter(30, time.Minute)
}
