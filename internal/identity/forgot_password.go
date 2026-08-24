package identity

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/mailtpl"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/notify"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/pwreset"
)

// forgotMessage is returned for every accepted request, whatever the address
// turns out to be. Any variation here — wording, status, header — would turn the
// endpoint into an account directory.
const forgotMessage = "Si un compte existe pour cette adresse, un email de réinitialisation a été envoyé."

// maxTokensPerAccountPerHour caps how often one account can be mailed, so a
// third party cannot flood someone's inbox by replaying the form. The caller
// still gets the ordinary answer: telling them they hit an per-account limit
// would confirm the account exists.
const maxTokensPerAccountPerHour = 3

// ForgotPassword issues a reset link for an account the caller claims to own.
//
// Every branch ends in the same 202, so the response reveals nothing about which
// addresses are registered. The work done before responding is deliberately
// similar in each branch, and the mail leaves in the background, because SMTP
// latency would otherwise make a hit trivially distinguishable from a miss.
func (h *Handlers) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email          string `json:"email"`
		TurnstileToken string `json:"turnstile_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	ip := middleware.ClientIP(r)
	// Not lower-cased: users.email is matched exactly by GetUserByEmail, so
	// folding case here would fail to find accounts that can still log in.
	email := strings.TrimSpace(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_email"})
		return
	}

	if h.turnstileEnforced {
		token := strings.TrimSpace(req.TurnstileToken)
		if token == "" {
			h.auditReset(0, email, "PASSWORD_RESET_BLOCKED_TURNSTILE", ip, "missing token")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "captcha_required"})
			return
		}
		if h.turnstile == nil {
			h.auditReset(0, email, "PASSWORD_RESET_BLOCKED_TURNSTILE", ip, "not configured")
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "captcha_failed"})
			return
		}
		if err := h.turnstile.Verify(r.Context(), token, ip); err != nil {
			h.auditReset(0, email, "PASSWORD_RESET_BLOCKED_TURNSTILE", ip, "verify failed")
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "captcha_failed"})
			return
		}
	}

	if h.resets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}

	h.issueResetIfEligible(email, ip)
	writeJSON(w, http.StatusAccepted, map[string]string{"message": forgotMessage})
}

// issueResetIfEligible performs the side effects and reports nothing: the caller
// answers identically either way. Every early return burns a token first so the
// ineligible paths cost roughly what the eligible one does.
func (h *Handlers) issueResetIfEligible(email, ip string) {
	user, err := h.users.GetUserByEmail(email)
	if err != nil {
		h.auditReset(0, email, "PASSWORD_RESET_REQUESTED", ip, "lookup error")
		return
	}
	if user == nil {
		pwreset.Burn()
		h.auditReset(0, email, "PASSWORD_RESET_REQUESTED", ip, "unknown address")
		return
	}
	// A forbidden account must not be recoverable: the link would hand back
	// access that an administrator deliberately withdrew.
	if domain.IsUserForbidden(user) {
		pwreset.Burn()
		h.auditReset(user.ID, user.Email, "PASSWORD_RESET_REQUESTED", ip, "account forbidden")
		return
	}

	recent, err := h.resets.CountRecentForUser(user.ID, time.Hour)
	if err == nil && recent >= maxTokensPerAccountPerHour {
		pwreset.Burn()
		h.auditReset(user.ID, user.Email, "PASSWORD_RESET_THROTTLED_ACCOUNT", ip,
			"already issued "+strconv.Itoa(recent)+" in the last hour")
		return
	}

	plain, expiresAt, err := h.resets.Issue(user.ID, ip)
	if err != nil {
		h.auditReset(user.ID, user.Email, "PASSWORD_RESET_REQUESTED", ip, "issue failed")
		return
	}
	h.auditReset(user.ID, user.Email, "PASSWORD_RESET_REQUESTED", ip, "reset link issued")

	h.dispatchMail(user, plain, expiresAt)
}

// dispatchMail hands delivery to the background so the response time carries no
// information about whether an address exists, nor about SMTP being slow.
func (h *Handlers) dispatchMail(user *domain.User, plain string, expiresAt time.Time) {
	if h.mailer == nil {
		return
	}
	send := func() {
		vars := h.resetVars(user, plain, expiresAt)
		if res := h.mailer.Send(domain.EmailSlugPasswordReset, user.Email, vars, nil, true); res.Err != nil {
			// The token stays valid: the user can ask again, and support can
			// still see the attempt in email_send_log.
			h.auditReset(user.ID, user.Email, "PASSWORD_RESET_EMAIL_FAILED", "", res.Err.Error())
		}
	}
	if h.dispatch != nil {
		h.dispatch(send)
		return
	}
	go send()
}

func (h *Handlers) resetVars(user *domain.User, plain string, expiresAt time.Time) notify.TemplateVars {
	vars := mailtpl.BaseUserVars(user)
	vars["reset_url"] = pwreset.BuildResetURL(pwreset.ResetLinkBaseURL(), plain)
	vars["expires_in"] = strconv.Itoa(pwreset.ExpiresInMinutes(expiresAt, time.Now()))
	// Declared rather than left to notify.Render's placeholder stripping, so a
	// template still carrying the pre-013 marker degrades to a blank instead of
	// depending on that behaviour staying put.
	vars["temporary_password"] = ""
	return vars
}
