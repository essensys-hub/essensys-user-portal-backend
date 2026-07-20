package identity

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/turnstile"
	"golang.org/x/crypto/bcrypt"
)

type Handlers struct {
	users             *data.UserStore
	turnstile         turnstile.Verifier
	turnstileEnforced bool
}

func NewHandlers(users *data.UserStore, verifier turnstile.Verifier, turnstileEnforced bool) *Handlers {
	return &Handlers{
		users:             users,
		turnstile:         verifier,
		turnstileEnforced: turnstileEnforced,
	}
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Email and password are required"})
		return
	}

	ip := middleware.ClientIP(r)

	// Honeypot: bots that fill hidden fields are rejected with a generic error.
	if strings.TrimSpace(req.Website) != "" {
		log.Printf("audit action=REGISTER_BLOCKED_HONEYPOT ip=%s", ip)
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid request"})
		return
	}

	if h.turnstileEnforced {
		token := strings.TrimSpace(req.TurnstileToken)
		if token == "" {
			log.Printf("audit action=REGISTER_BLOCKED_TURNSTILE ip=%s reason=missing_token", ip)
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Captcha verification required"})
			return
		}
		if h.turnstile == nil {
			log.Printf("audit action=REGISTER_BLOCKED_TURNSTILE ip=%s reason=not_configured", ip)
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "Captcha verification failed"})
			return
		}
		if err := h.turnstile.Verify(r.Context(), token, ip); err != nil {
			log.Printf("audit action=REGISTER_BLOCKED_TURNSTILE ip=%s reason=verify_failed", ip)
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "Captcha verification failed"})
			return
		}
	}

	existing, err := h.users.GetUserByEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Database error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"message": "User already exists"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Failed to process password"})
		return
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hashed),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Provider:     domain.ProviderEmail,
		Role:         domain.RoleGuestLocal,
		CreatedAt:    time.Now(),
		LastLogin:    time.Now(),
	}
	if err := h.users.CreateUser(user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Failed to create user"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": "User registered successfully"})
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.users.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if domain.IsUserForbidden(user) {
		domain.WriteAccountForbidden(w)
		return
	}
	if user.Provider != domain.ProviderEmail && user.PasswordHash == "" {
		http.Error(w, "Please login with "+user.Provider, http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	_ = h.users.UpdateLastLogin(user.ID)
	token, err := middleware.GenerateJWT(user.Email, user.Role, time.Now().Add(24*time.Hour))
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  domain.UserToResponse(user),
	})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetProfile(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.users.GetUserByEmail(email)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, domain.UserToResponse(user))
}

func (h *Handlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.users.GetUserByEmail(email)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	hash := ""
	if req.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Server Error", http.StatusInternalServerError)
			return
		}
		hash = string(hashed)
	}
	if err := h.users.UpdateUser(user.ID, req.FirstName, req.LastName, hash); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.users.GetUserByEmail(email)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err := h.users.DeleteUser(user.ID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) UpdateProfileLinks(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.users.GetUserByEmail(email)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	var req struct {
		MachineID *int    `json:"linked_machine_id"`
		GatewayID *string `json:"linked_gateway_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := h.users.UpdateUserLinks(user.ID, req.MachineID, req.GatewayID, nil); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) NearbyDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"machines": []any{},
		"gateways": []any{},
		"user_ip":  middleware.ClientIP(r),
	})
}
