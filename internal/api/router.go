package api

import (
	"net/http"
	"os"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(store *data.PortalStore) http.Handler {
	h := NewHandler(store)
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	allowedOrigin := os.Getenv("CORS_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "https://mon.essensys.fr"
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{allowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Gateway-ID", "X-Gateway-Eth0-MAC", "X-Gateway-Eth1-MAC"},
		AllowCredentials: true,
	}))

	injectLimiter := middleware.NewRateLimiter(30, time.Minute)

	r.Get("/api/portal/health", h.Health)

	r.Route("/api/portal", func(r chi.Router) {
		r.Use(middleware.UserJWT)
		r.Post("/link-request", h.CreateLinkRequest)
		r.Get("/link-request/status", h.LinkRequestStatus)
		r.Get("/gateway/status", h.GatewayStatus)
		r.With(injectLimiter.Middleware).Post("/inject", h.Inject)

		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.AdminJWT)
			r.Get("/link-requests", h.ListPendingLinkRequests)
			r.Put("/link-requests/{id}", h.ReviewLinkRequest)
			r.Get("/gateway-sessions", h.ListGatewaySessions)
			r.Post("/gateways/register", h.RegisterGateway)
		})
	})

	gatewayAuth := middleware.GatewayAuth(store)
	r.Route("/api/gateway", func(r chi.Router) {
		r.With(gatewayAuth).Get("/pending-actions", h.PendingActions)
		r.With(gatewayAuth).Post("/actions/{guid}/done", h.ActionDone)
		r.With(gatewayAuth).Post("/heartbeat", h.Heartbeat)
	})

	return r
}
