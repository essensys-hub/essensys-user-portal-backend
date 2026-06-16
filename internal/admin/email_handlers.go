package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/notify"
	"github.com/go-chi/chi/v5"
)

func (h *Handlers) EmailHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": notify.Configured(),
		"from":       os.Getenv("SMTP_FROM"),
	})
}

func (h *Handlers) ListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	if h.templates == nil {
		writeJSON(w, http.StatusOK, []domain.EmailTemplate{})
		return
	}
	list, err := h.templates.List()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) GetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	slug := chi.URLParam(r, "slug")
	tpl, err := h.templates.Get(slug)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (h *Handlers) PutEmailTemplate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	slug := chi.URLParam(r, "slug")
	var req domain.EmailTemplate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	req.Slug = slug
	if req.Name == "" {
		req.Name = slug
	}
	if err := h.templates.Upsert(&req); err != nil {
		http.Error(w, "Failed to save template", http.StatusInternalServerError)
		return
	}
	tpl, _ := h.templates.Get(slug)
	writeJSON(w, http.StatusOK, tpl)
}

func (h *Handlers) PreviewEmailTemplate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	slug := chi.URLParam(r, "slug")
	tpl, err := h.templates.Get(slug)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	vars := notify.SampleTemplateVars()
	var body struct {
		Vars notify.TemplateVars `json:"vars"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.Vars) > 0 {
		vars = body.Vars
	}
	subject := notify.Render(tpl.Subject, vars)
	html := notify.Render(tpl.BodyHTML, vars)
	if html == "" {
		html = notify.Render(tpl.BodyText, vars)
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"subject":   subject,
		"body_html": html,
	})
}

func (h *Handlers) TestEmailTemplate(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.requireAdminGlobal(w, r)
	if !ok {
		return
	}
	var req struct {
		TemplateSlug string `json:"template_slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TemplateSlug == "" {
		http.Error(w, "template_slug required", http.StatusBadRequest)
		return
	}
	tpl, err := h.templates.Get(req.TemplateSlug)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if !notify.Configured() {
		http.Error(w, "SMTP configuration missing", http.StatusServiceUnavailable)
		return
	}
	vars := notify.SampleTemplateVars()
	vars["email"] = admin.Email
	subject := "[TEST] " + notify.Render(tpl.Subject, vars)
	body := notify.Render(tpl.BodyHTML, vars)
	if body == "" {
		body = notify.Render(tpl.BodyText, vars)
	}
	if err := notify.Send([]string{admin.Email}, subject, body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent", "to": admin.Email})
}

func (h *Handlers) ResendUserEmail(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.requireAdminGlobal(w, r)
	if !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}
	user, err := h.users.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	var req struct {
		TemplateSlug string `json:"template_slug"`
		Password     string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TemplateSlug == "" {
		http.Error(w, "template_slug required", http.StatusBadRequest)
		return
	}
	result := h.sendTemplateEmail(req.TemplateSlug, user, req.Password, admin.ID, admin.Email, clientIP(r))
	if result.Err != nil {
		http.Error(w, result.Err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": result.Sent, "email": user.Email})
}

func (h *Handlers) requireAdminGlobal(w http.ResponseWriter, r *http.Request) (*domain.User, bool) {
	if h.users == nil {
		http.Error(w, "User Store not initialized", http.StatusServiceUnavailable)
		return nil, false
	}
	email, _ := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.users.GetUserByEmail(email)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if user.Role != domain.RoleAdminGlobal {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}
	return user, true
}
