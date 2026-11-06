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
	"github.com/essensys-hub/essensys-user-portal-backend/internal/remember"
)

const maxUserAgentLen = 255

// issueRememberSession creates the 30-day session. expires_at is fixed here and
// is never written again: neither a refresh nor any other use extends it.
func (h *Handlers) issueRememberSession(userID int, r *http.Request) (plain string, expiresAt time.Time, err error) {
	plain, digest, err := remember.Generate()
	if err != nil {
		return "", time.Time{}, err
	}
	now := h.now()
	expiresAt = now.Add(remember.Lifetime)
	ua := r.UserAgent()
	if len(ua) > maxUserAgentLen {
		ua = ua[:maxUserAgentLen]
	}
	if err := h.remember.Create(userID, digest, expiresAt, ua, now); err != nil {
		return "", time.Time{}, err
	}
	return plain, expiresAt, nil
}

// rememberInvalid is the one answer for an unknown, expired or revoked token,
// so the response never tells a caller which of the three it was.
func rememberInvalid(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "remember_invalid"})
}

// RememberRefresh swaps a valid remembered session for a fresh 24 h JWT. It
// reads the account again, so a user forbidden since the login is refused.
func (h *Handlers) RememberRefresh(w http.ResponseWriter, r *http.Request) {
	if h.remember == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	var req struct {
		RememberToken string `json:"remember_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	plain := strings.TrimSpace(req.RememberToken)
	if plain == "" {
		rememberInvalid(w)
		return
	}

	ip := middleware.ClientIP(r)
	sess, err := h.remember.GetByHash(remember.Hash(plain))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	now := h.now()
	if !sess.Usable(now) {
		log.Printf("audit action=REMEMBER_REFRESH_REFUSED ip=%s", ip)
		rememberInvalid(w)
		return
	}

	user, err := h.users.GetUserByID(sess.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	if user == nil {
		rememberInvalid(w)
		return
	}
	if domain.IsUserForbidden(user) {
		log.Printf("audit action=REMEMBER_REFRESH_REFUSED user_id=%d ip=%s reason=account_forbidden", user.ID, ip)
		domain.WriteAccountForbidden(w)
		return
	}

	token, err := middleware.GenerateJWT(user.Email, user.Role, now.Add(24*time.Hour))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	// Best effort: the last use is informative, and a failure here must not
	// lock out a user whose session is valid.
	if err := h.remember.Touch(sess.ID, now); err != nil {
		log.Printf("remember: touch session id=%d: %v", sess.ID, err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":                    token,
		"user":                     domain.UserToResponse(user),
		"password_change_required": domain.PasswordChangeRequired(user),
	})
}

// Logout ends the remembered session named in the optional body. It always
// answers 200, as it always did: an app signing out must never be told it
// failed, and a call without a body leaves other sessions untouched.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	defer w.WriteHeader(http.StatusOK)
	if h.remember == nil || r.Body == nil {
		return
	}
	var req struct {
		RememberToken string `json:"remember_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return
	}
	plain := strings.TrimSpace(req.RememberToken)
	if plain == "" {
		return
	}
	if err := h.remember.RevokeByHash(remember.Hash(plain), h.now()); err != nil {
		log.Printf("remember: revoke on logout: %v", err)
		return
	}
	h.auditRemember(0, "", "REMEMBER_SESSION_REVOKED", middleware.ClientIP(r), "remembered session revoked at logout")
}

// auditRemember never receives a token: only the action and who asked.
func (h *Handlers) auditRemember(userID int, email, action, ip, details string) {
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
		CreatedAt:    h.now(),
	})
}
