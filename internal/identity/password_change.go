package identity

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/pwreset"
	"golang.org/x/crypto/bcrypt"
)

// ChangePassword is the one route mounted under
// middleware.UserJWTAllowPasswordChange rather than UserJWTWithStore — the
// deliberate exemption from the password-change-required lock (D3), because
// it is the only way out of that lock. Requiring the caller's current
// password rather than trusting the JWT alone means a browser left open on a
// shared machine cannot be used to silently replace the password.
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	email, _ := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.users.GetUserByEmail(email)
	if err != nil || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)) != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_current_password"})
		return
	}
	if len([]rune(req.NewPassword)) < pwreset.MinPasswordLength {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "weak_password",
			"message": "Le mot de passe doit contenir au moins 8 caractères.",
		})
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "password_reused",
			"message": "Choisissez un mot de passe différent de l'ancien.",
		})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}

	if err := h.users.ClearPasswordChangeRequired(user.ID, string(hashed)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}

	h.auditPasswordChanged(user.ID, user.Email, middleware.ClientIP(r))
	writeJSON(w, http.StatusOK, map[string]string{"message": "Mot de passe mis à jour."})
}

func (h *Handlers) auditPasswordChanged(userID int, email, ip string) {
	log.Printf("audit action=PASSWORD_CHANGED_AFTER_TEMPORARY user_id=%d ip=%s", userID, ip)
	if h.audit == nil {
		return
	}
	_ = h.audit.CreateAuditLog(&domain.AuditLog{
		UserID:       userID,
		Username:     email,
		Action:       "PASSWORD_CHANGED_AFTER_TEMPORARY",
		ResourceType: "USER",
		ResourceID:   strconv.Itoa(userID),
		IPAddress:    ip,
		Details:      "Password changed, lock cleared",
		CreatedAt:    time.Now(),
	})
}
