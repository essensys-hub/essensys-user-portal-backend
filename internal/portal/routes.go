package portal

import (
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/handlers"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Mount registers user portal routes (/api/portal/*).
func Mount(r chi.Router, h *handlers.Handler, users *data.UserStore, injectLimiter *middleware.RateLimiter) {
	r.Route("/portal", func(r chi.Router) {
		r.Get("/health", h.Health)

		r.Group(func(r chi.Router) {
			r.Use(middleware.UserJWTWithStore(users))
		r.Post("/link-request", h.CreateLinkRequest)
		r.Get("/link-request/status", h.LinkRequestStatus)
		r.Get("/session", h.PortalSession)
		r.Get("/gateway/status", h.GatewayStatus)
		r.Put("/push/devices", h.PutPushDevice)
		r.Delete("/push/devices/{id}", h.DeletePushDevice)
		r.Get("/doorbell/preferences", h.GetDoorbellPreferences)
		r.With(injectLimiter.Middleware).Post("/inject", h.Inject)
		r.With(injectLimiter.Middleware).Post("/inject/batch", h.InjectBatch)
		r.Get("/exchange", h.GetExchange)
		r.Get("/history/latest", h.GetHistoryLatest)
		r.Post("/web/actions", h.PostWebActions)
		r.Get("/lights/timers", h.GetLightTimers)
		r.Get("/lights/names", h.GetLightNames)
		r.Get("/shutters/names", h.GetShutterNames)
		r.Get("/lights/layout", h.GetLightsLayout)
		r.Get("/shutters/layout", h.GetShuttersLayout)

		r.Get("/scenarios", h.ListScenarios)
		r.Get("/scenarios/meta/bitmasks", h.GetScenarioBitmasks)
		r.Get("/scenarios/{slot}", h.GetScenario)
		r.Put("/scenarios/{slot}", h.PutScenario)
		r.With(injectLimiter.Middleware).Post("/scenarios/{slot}/launch", h.LaunchScenario)
		r.With(injectLimiter.Middleware).Post("/scenarios/{slot}/restore", h.RestoreScenario)

		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.AdminJWTWithStore(users))
			r.Get("/link-requests", h.ListPendingLinkRequests)
			r.Put("/link-requests/{id}", h.ReviewLinkRequest)
			r.Get("/gateway-sessions", h.ListGatewaySessions)
			r.Post("/gateways/register", h.RegisterGateway)
			r.Put("/lights/timers/{id}", h.PutLightTimer)
			r.Put("/lights/names/{id}", h.PutLightName)
			r.Put("/shutters/names/{id}", h.PutShutterName)
			r.Put("/lights/layout", h.PutLightsLayout)
			r.Put("/shutters/layout", h.PutShuttersLayout)
		})
		})
	})
}

// DefaultInjectLimiter matches production rate limit for inject.
func DefaultInjectLimiter() *middleware.RateLimiter {
	return middleware.NewRateLimiter(30, time.Minute)
}
