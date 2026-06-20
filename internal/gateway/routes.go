package gateway

import (
	"github.com/essensys-hub/essensys-user-portal-backend/internal/handlers"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Mount registers gateway agent routes (/api/gateway/*).
func Mount(r chi.Router, h *handlers.Handler, store *data.PortalStore) {
	auth := middleware.GatewayAuth(store)
	r.Route("/gateway", func(r chi.Router) {
		r.With(auth).Get("/pending-actions", h.PendingActions)
		r.With(auth).Post("/actions/{guid}/done", h.ActionDone)
		r.With(auth).Post("/heartbeat", h.Heartbeat)
		r.With(auth).Post("/exchange", h.PushExchange)
		r.With(auth).Get("/sync-config", h.SyncConfig)
		r.With(auth).Post("/sync-runs", h.SyncRunCreate)
		r.With(auth).Post("/sync-runs/{id}/start", h.SyncRunStart)
		r.With(auth).Post("/sync-runs/{id}/progress", h.SyncRunProgress)
		r.With(auth).Patch("/sync-profiles/scenarios", h.PatchScenariosSyncProfile)
	})
}
