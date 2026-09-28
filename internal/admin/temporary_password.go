package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/notify"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/pwreset"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/temppass"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// temporaryPasswordValidity is the fixed lifetime of an admin-issued
// temporary password (D1). Not configurable per issue: a manual dependability
// dial here is more surface than the feature needs, and the fixed value is
// what the UI's confirmation text promises the administrator.
const temporaryPasswordValidity = 72 * time.Hour

type issueTemporaryPasswordRequest struct {
	SendEmail bool `json:"send_email"`
}

// IssueTemporaryPassword lets support unblock a user who cannot receive
// mail. Unlike SendPasswordReset, the plaintext leaves the server exactly
// once — in this response — so the administrator can read it aloud and
// relay it by phone; email delivery is opt-in per D7.
func (h *Handlers) IssueTemporaryPassword(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.requireAdminGlobal(w, r)
	if !ok {
		return
	}
	if h.users == nil {
		http.Error(w, "User Store not initialized", http.StatusServiceUnavailable)
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
	// Same rule as SendPasswordReset: issuing a fresh credential to a
	// forbidden account would quietly reopen it. Lifting the ban has to be a
	// separate, deliberate action.
	if domain.IsUserForbidden(user) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "account_forbidden"})
		return
	}

	var req issueTemporaryPasswordRequest
	if r.Body != nil {
		// A missing or empty body means "don't send" (the default), not a
		// bad request — most callers only want the on-screen password.
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	plain, err := temppass.Generate()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(temporaryPasswordValidity)
	if err := h.users.SetTemporaryPassword(user.ID, string(hash), expiresAt, admin.ID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	ip := clientIP(r)

	// A live reset link and a temporary password are two concurrent ways
	// into the same account; the one just issued supersedes the other (D6).
	if h.resets != nil {
		_ = h.resets.InvalidateForUser(user.ID)
	}

	// The plaintext is logged nowhere: not here, not inside logAudit.
	h.logAudit(admin.ID, admin.Email, "TEMPORARY_PASSWORD_ISSUED", "USER", idStr, ip,
		fmt.Sprintf("Temporary password issued for %s, expires_at=%s", user.Email, expiresAt.UTC().Format(time.RFC3339)))

	resp := map[string]any{
		"password":   plain,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
		"email_sent": false,
	}

	if req.SendEmail {
		result := h.sendTemplateEmailWithVars(domain.EmailSlugTemporaryPassword, user, plain, notify.TemplateVars{
			"login_url":  loginURL(),
			"expires_in": formatValidity(temporaryPasswordValidity),
		}, admin.ID, admin.Email, ip, true)
		resp["email_sent"] = result.Sent
		if result.Err != nil {
			resp["email_sent"] = false
			resp["reason"] = result.Err.Error()
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// loginURL points at the support site's login page, not mailtpl.PortalURL():
// that reads FRONTEND_URL, which in production is mon.essensys.fr — the
// portal SPA, which has no /login route. pwreset.ResetLinkBaseURL() already
// solves exactly this for the reset-link email; reused here rather than
// duplicated.
func loginURL() string {
	return pwreset.ResetLinkBaseURL() + "/login"
}

func formatValidity(d time.Duration) string {
	hours := int(d.Hours())
	if hours == 1 {
		return "1 heure"
	}
	return fmt.Sprintf("%d heures", hours)
}
