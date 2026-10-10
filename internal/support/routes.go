package support

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// UserStore resolves the signed-in account.
type UserStore interface {
	GetUserByEmail(email string) (*domain.User, error)
}

// Mount registers /support/reports behind the same session checks as the
// rest of the portal: 401 without a valid session, 403 for a forbidden
// account, 409 while a temporary password is pending.
func Mount(r chi.Router, users UserStore, svc *Service) {
	h := &handler{users: users, svc: svc}
	r.Group(func(r chi.Router) {
		r.Use(middleware.UserJWTWithStore(users))
		r.Post("/support/reports", h.create)
		r.Get("/support/reports", h.list)
	})
}

type handler struct {
	users UserStore
	svc   *Service
}

var fieldMessages = map[string]string{
	"required": "Ce champ est obligatoire.",
	"too_long": "Ce texte est trop long.",
	"invalid":  "Valeur non reconnue.",
}

var sensitiveMessages = map[string]string{
	CategoryPassword:      "Ce texte semble contenir un mot de passe. Retirez-le : nous ne vous le demanderons jamais.",
	CategorySecret:        "Ce texte semble contenir un code secret ou une clé. Retirez-le avant d'envoyer.",
	CategoryPostalAddress: "Ce texte semble contenir une adresse postale. Retirez-la : elle n'est pas utile pour traiter le problème.",
	CategoryEmail:         "Ce texte contient une adresse email. Retirez-la : nous vous répondrons via votre compte.",
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *handler) currentUser(w http.ResponseWriter, r *http.Request) *domain.User {
	email, _ := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.users.GetUserByEmail(email)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}
	return user
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(w, r)
	if user == nil {
		return
	}
	var in Input
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json", "message": "Requête illisible."})
		return
	}
	rep, err := h.svc.Submit(r.Context(), user, in)
	var fe *FieldError
	var se *SensitiveError
	var rl *RateLimitError
	switch {
	case errors.As(err, &fe):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_field", "field": fe.Field, "reason": fe.Reason, "message": fieldMessages[fe.Reason]})
	case errors.As(err, &se):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "sensitive_content", "field": se.Field, "category": se.Category, "message": sensitiveMessages[se.Category]})
	case errors.As(err, &rl):
		secs := int(math.Ceil(time.Until(rl.RetryAt).Seconds()))
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited", "retry_after": secs, "retry_at": rl.RetryAt.UTC().Format(time.RFC3339),
			"message": "Vous avez atteint la limite de 5 signalements par 24 heures."})
	case err != nil:
		log.Printf("support: create: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	default:
		writeJSON(w, http.StatusCreated, view(*rep))
	}
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(w, r)
	if user == nil {
		return
	}
	reps, err := h.svc.List(r.Context(), user)
	if err != nil {
		log.Printf("support: list: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	out := make([]reportView, 0, len(reps))
	for _, rep := range reps {
		out = append(out, view(rep))
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": out})
}

type reportView struct {
	Ref       string    `json:"ref"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	IssueURL  string    `json:"issue_url,omitempty"`
}

// view never exposes an incident's issue URL: it would reveal the private
// repository.
func view(r data.SupportReport) reportView {
	v := reportView{Ref: r.PublicRef, Kind: r.Kind, Title: r.Title, Status: r.Status, CreatedAt: r.CreatedAt}
	if r.Kind == KindBug && r.IssueURL.Valid {
		v.IssueURL = r.IssueURL.String
	}
	return v
}
