package api

import (
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/admin"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	gw "github.com/essensys-hub/essensys-user-portal-backend/internal/gateway"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/handlers"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/identity"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/legacyiot"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/portal"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/newrelic/go-agent/v3/integrations/nrgochi"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func NewRouter(store *data.PortalStore, users *data.UserStore, audit *data.AuditStore, inventory *data.AdminInventoryStore, news *data.NewsletterStore, templates *data.EmailTemplateStore, iot *data.LegacyIoTStore, nrApp *newrelic.Application, cfg config.Config) http.Handler {
	h := handlers.NewHandler(store, inventory, cfg.ExchangeStaleTTL)
	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Gateway-ID", "X-Gateway-Eth0-MAC", "X-Gateway-Eth1-MAC"},
		AllowCredentials: true,
	}))

	injectLimiter := portal.DefaultInjectLimiter()

	if nrApp != nil {
		r.Use(nrgochi.Middleware(nrApp))
	}

	r.Route("/api", func(r chi.Router) {
		portal.Mount(r, h, injectLimiter)
		gw.Mount(r, h, store)

		if cfg.ConsolidatedMode {
			identity.Mount(r, users)
			admin.Mount(r, admin.Deps{
				Users:     users,
				Audit:     audit,
				Inventory: inventory,
				News:      news,
				Templates: templates,
				Portal:    store,
			})
			legacyiot.Mount(r, iot, store)
		}
	})

	return r
}
