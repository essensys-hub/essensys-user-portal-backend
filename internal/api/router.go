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
	"github.com/essensys-hub/essensys-user-portal-backend/internal/mailtpl"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/portal"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/newrelic/go-agent/v3/integrations/nrgochi"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func NewRouter(store *data.PortalStore, users *data.UserStore, audit *data.AuditStore, inventory *data.AdminInventoryStore, news *data.NewsletterStore, templates *data.EmailTemplateStore, iot *data.LegacyIoTStore, resets *data.PasswordResetStore, nrApp *newrelic.Application, cfg config.Config) http.Handler {
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
		portal.Mount(r, h, users, injectLimiter)
		gw.Mount(r, h, store)

		if cfg.ConsolidatedMode {
			identityOpts := []identity.Option{
				identity.WithPasswordResets(resets),
				identity.WithAudit(audit),
			}
			// Without the mailer the forgot endpoint still issues tokens but
			// nothing reaches the user, so only wire it when templates exist.
			if templates != nil {
				identityOpts = append(identityOpts, identity.WithMailer(mailtpl.NewSender(templates)))
			}
			identity.Mount(r, users, cfg, identityOpts...)
			admin.Mount(r, admin.Deps{
				Users:     users,
				Audit:     audit,
				Inventory: inventory,
				IoT:       iot,
				News:      news,
				Templates: templates,
				Portal:    store,
				Resets:    resets,
			})
			legacyiot.Mount(r, iot, store)
		}
	})

	return r
}
