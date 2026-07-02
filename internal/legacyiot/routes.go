package legacyiot

import (
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func Mount(r chi.Router, store *data.LegacyIoTStore, portal *data.PortalStore) {
	if store == nil {
		return
	}
	h := NewHandlers(store, portal)

	r.Group(func(r chi.Router) {
		r.Use(middleware.BasicAuth(store, true))
		r.Get("/myactions", h.MyActions)
		r.Post("/done/{guid}", h.Done)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.BasicAuth(store, false))
		r.Get("/serverinfos", h.ServerInfos)
		r.Post("/mystatus", h.MyStatus)
		r.Post("/infos", h.GatewayInfos)
	})
}
