package admin

import (
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func Mount(r chi.Router, d Deps) {
	if d.Users == nil {
		return
	}
	h := NewHandlers(d)

	r.Post("/newsletter/subscribe", h.Subscribe)
	r.Post("/admin/login", h.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.AdminAuth)
		r.Get("/admin/stats", h.Stats)
		r.Get("/admin/audit", h.AuditLogs)
		r.Get("/admin/machines", h.Machines)
		r.Get("/admin/gateways", h.Gateways)
		r.Get("/admin/subscribers", h.Subscribers)
		r.Post("/admin/subscribers", h.AddSubscriber)
		r.Delete("/admin/subscribers", h.DeleteSubscriber)
		r.Get("/admin/newsletters", h.GetNewsletters)
		r.Post("/admin/newsletters", h.CreateNewsletter)
		r.Put("/admin/newsletters/{id}", h.UpdateNewsletter)
		r.Delete("/admin/newsletters/{id}", h.DeleteNewsletter)
		r.Post("/admin/newsletters/{id}/send", h.SendNewsletter)
		r.Get("/admin/users", h.GetUsers)
		r.Post("/admin/users", h.CreateUser)
		r.Put("/admin/users/{id}/role", h.UpdateUserRole)
		r.Put("/admin/users/{id}/links", h.UpdateUserLinks)
		r.Post("/admin/users/{id}/resend-email", h.ResendUserEmail)
		r.Get("/admin/email/health", h.EmailHealth)
		r.Get("/admin/email-templates", h.ListEmailTemplates)
		r.Get("/admin/email-templates/{slug}", h.GetEmailTemplate)
		r.Put("/admin/email-templates/{slug}", h.PutEmailTemplate)
		r.Post("/admin/email-templates/{slug}/preview", h.PreviewEmailTemplate)
		r.Post("/admin/email-templates/test", h.TestEmailTemplate)
	})
}
