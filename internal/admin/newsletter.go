package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/notify"
	"github.com/go-chi/chi/v5"
)

type subscribeRequest struct {
	Email string `json:"email"`
}

func (h *Handlers) Subscribe(w http.ResponseWriter, r *http.Request) {
	if h.news == nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		http.Error(w, "Email required", http.StatusBadRequest)
		return
	}
	if err := h.news.AddSubscriber(req.Email); err != nil {
		log.Printf("[newsletter] subscribe: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "subscribed"})
}

func (h *Handlers) Subscribers(w http.ResponseWriter, r *http.Request) {
	if h.news == nil {
		writeJSON(w, http.StatusOK, []domain.Subscriber{})
		return
	}
	subs, err := h.news.GetSubscribers()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handlers) AddSubscriber(w http.ResponseWriter, r *http.Request) {
	if h.news == nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		http.Error(w, "Email required", http.StatusBadRequest)
		return
	}
	if err := h.news.AddSubscriber(req.Email); err != nil {
		http.Error(w, "Failed to add", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) DeleteSubscriber(w http.ResponseWriter, r *http.Request) {
	if h.news == nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email query param required", http.StatusBadRequest)
		return
	}
	if err := h.news.DeleteSubscriber(email); err != nil {
		http.Error(w, "Failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetNewsletters(w http.ResponseWriter, r *http.Request) {
	if h.news == nil {
		writeJSON(w, http.StatusOK, []domain.Newsletter{})
		return
	}
	list, err := h.news.GetNewsletters()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) CreateNewsletter(w http.ResponseWriter, r *http.Request) {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	n := domain.Newsletter{
		ID:        id,
		Subject:   "Nouvelle Newsletter",
		Content:   "# Nouveau Brouillon\n\nCommencez à rédiger ici...",
		Status:    "draft",
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.news.SaveNewsletter(n); err != nil {
		http.Error(w, "Failed to create", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *Handlers) UpdateNewsletter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	current, err := h.news.GetNewsletter(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if current.Status == "sent" {
		http.Error(w, "Cannot edit sent newsletter", http.StatusForbidden)
		return
	}
	var req domain.Newsletter
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	current.Subject = req.Subject
	current.Content = req.Content
	current.UpdatedAt = time.Now()
	current.Version++
	if req.Status == "draft" || req.Status == "ready" {
		current.Status = req.Status
	}
	if err := h.news.SaveNewsletter(*current); err != nil {
		http.Error(w, "Failed to save", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, current)
}

func (h *Handlers) DeleteNewsletter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.news.DeleteNewsletter(id); err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) SendNewsletter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	n, err := h.news.GetNewsletter(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if n.Status != "ready" && n.Status != "sent" {
		http.Error(w, "Newsletter must be 'ready' or 'sent' to send", http.StatusBadRequest)
		return
	}

	subs, _ := h.news.GetSubscribers()
	successCount, failCount := 0, 0
	for _, s := range subs {
		if err := notify.Send([]string{s.Email}, n.Subject, n.Content); err != nil {
			log.Printf("[newsletter] send to %s: %v", s.Email, err)
			failCount++
		} else {
			successCount++
		}
	}
	if successCount == 0 && failCount > 0 && os.Getenv("SMTP_HOST") == "" {
		http.Error(w, "SMTP Configuration Missing", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	n.Status = "sent"
	n.SentAt = &now
	n.UpdatedAt = now
	if err := h.news.SaveNewsletter(*n); err != nil {
		http.Error(w, "Failed to save status", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, n)
}
