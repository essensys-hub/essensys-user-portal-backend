package identity

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/pwreset"
	"golang.org/x/crypto/bcrypt"
)

// ValidateResetToken lets the UI decide whether to render the password fields at
// all. It is deliberately free of side effects: mail scanners and antivirus
// prefetchers follow links with GET, and consuming here would burn the token
// before the user ever saw the form.
func (h *Handlers) ValidateResetToken(w http.ResponseWriter, r *http.Request) {
	if h.resets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"valid": false, "reason": "unavailable"})
		return
	}
	plain := strings.TrimSpace(r.URL.Query().Get("token"))
	if plain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"valid": false, "reason": "invalid"})
		return
	}

	tok, err := h.resets.GetByHash(pwreset.Hash(plain))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"valid": false, "reason": "error"})
		return
	}
	if tok == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"valid": false, "reason": "invalid"})
		return
	}

	switch tok.State(time.Now()) {
	case domain.PasswordResetTokenExpired:
		writeJSON(w, http.StatusGone, map[string]any{"valid": false, "reason": "expired"})
		return
	case domain.PasswordResetTokenUsed, domain.PasswordResetTokenInvalidated:
		writeJSON(w, http.StatusGone, map[string]any{"valid": false, "reason": "used"})
		return
	}

	user, err := h.users.GetUserByID(tok.UserID)
	if err != nil || user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"valid": false, "reason": "invalid"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":        true,
		"email_masked": pwreset.MaskEmail(user.Email),
	})
}

// ResetPassword consumes the token and writes the new credential. Ordering
// matters: every rejection that is the user's fault (too short, reused) happens
// before the consume, so a mistyped password does not cost them the link.
func (h *Handlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if h.resets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}

	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	ip := middleware.ClientIP(r)
	plain := strings.TrimSpace(req.Token)
	if plain == "" {
		h.auditReset(0, "", "PASSWORD_RESET_TOKEN_INVALID", ip, "empty token")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_token"})
		return
	}

	tokenHash := pwreset.Hash(plain)
	tok, err := h.resets.GetByHash(tokenHash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	if tok == nil || !tok.Usable(time.Now()) {
		h.auditReset(0, "", "PASSWORD_RESET_TOKEN_INVALID", ip, "token not usable")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_token"})
		return
	}

	user, err := h.users.GetUserByID(tok.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	if user == nil {
		h.auditReset(0, "", "PASSWORD_RESET_TOKEN_INVALID", ip, "user missing")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_token"})
		return
	}

	// An account forbidden after the link was issued must not be reopened by it.
	if domain.IsUserForbidden(user) {
		_ = h.resets.InvalidateForUser(user.ID)
		h.auditReset(user.ID, user.Email, "PASSWORD_RESET_TOKEN_INVALID", ip, "account forbidden")
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "account_forbidden"})
		return
	}

	if len([]rune(req.Password)) < pwreset.MinPasswordLength {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "weak_password",
			"message": "Le mot de passe doit contenir au moins 8 caractères.",
		})
		return
	}
	if user.PasswordHash != "" &&
		bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "password_reused",
			"message": "Choisissez un mot de passe différent de l'ancien.",
		})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}

	userID, err := h.resets.ConsumeAndSetPassword(tokenHash, string(hashed), ip)
	if err != nil {
		// Lost the race against a concurrent consume, or the token expired
		// between the read above and here. Same opaque answer either way.
		h.auditReset(user.ID, user.Email, "PASSWORD_RESET_TOKEN_INVALID", ip, "consume failed")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_token"})
		return
	}

	h.auditReset(userID, user.Email, "PASSWORD_RESET_COMPLETED", ip, "password updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Mot de passe mis à jour."})
}

func (h *Handlers) auditReset(userID int, email, action, ip, details string) {
	log.Printf("audit action=%s user_id=%d ip=%s", action, userID, ip)
	if h.audit == nil {
		return
	}
	_ = h.audit.CreateAuditLog(&domain.AuditLog{
		UserID:       userID,
		Username:     email,
		Action:       action,
		ResourceType: "USER",
		ResourceID:   strconv.Itoa(userID),
		IPAddress:    ip,
		Details:      details,
		CreatedAt:    time.Now(),
	})
}
