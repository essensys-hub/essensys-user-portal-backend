package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/notify"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/pwreset"
	"github.com/go-chi/chi/v5"
)

// SendPasswordReset lets support unblock a user who cannot get past the login
// form. It emits the same single-use token as the self-service flow: the
// administrator never sees it, so this cannot be used to take over an account.
func (h *Handlers) SendPasswordReset(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.requireAdminGlobal(w, r)
	if !ok {
		return
	}
	if h.resets == nil {
		http.Error(w, "Password reset store not initialized", http.StatusServiceUnavailable)
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
	// Sending a reset to a forbidden account would quietly reopen it; lifting
	// the ban has to be a separate, deliberate action.
	if domain.IsUserForbidden(user) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "account_forbidden"})
		return
	}

	ip := clientIP(r)
	plain, expiresAt, err := h.resets.Issue(user.ID, ip)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logAudit(admin.ID, admin.Email, "PASSWORD_RESET_REQUESTED_BY_ADMIN", "USER", idStr, ip,
		"Reset link issued for "+user.Email)

	result := h.sendTemplateEmailWithVars(domain.EmailSlugPasswordReset, user, "", notify.TemplateVars{
		"reset_url":  pwreset.BuildResetURL(pwreset.PortalBaseURL(), plain),
		"expires_in": strconv.Itoa(pwreset.ExpiresInMinutes(expiresAt, time.Now())),
		// Pinned empty so a template still carrying the old marker cannot fall
		// back to the "contact your administrator" filler mid-sentence.
		"temporary_password": "",
	}, admin.ID, admin.Email, ip, true)

	// The token stays valid even when delivery fails, so the operator can retry
	// or fall back to another channel. Report the reason rather than a 500: the
	// action itself succeeded, only the delivery did not.
	if result.Err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"email_sent": false,
			"reason":     result.Err.Error(),
			"expires_at": expiresAt.UTC().Format(time.RFC3339),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"email_sent": result.Sent,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}
