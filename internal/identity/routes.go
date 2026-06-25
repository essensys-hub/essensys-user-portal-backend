package identity

import (
	"encoding/json"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func Mount(r chi.Router, users *data.UserStore) {
	if users == nil {
		return
	}
	h := NewHandlers(users)

	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
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
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
