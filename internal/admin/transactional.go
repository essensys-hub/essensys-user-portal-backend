package admin

import (
	"fmt"
	"strings"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/mailtpl"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/notify"
)

const passwordFallback = "contactez votre administrateur"

type sendResult struct {
	Sent bool
	Err  error
}

func (h *Handlers) tryAutoSend(slug string, user *domain.User, tempPassword string, adminID int, adminEmail, ip string) sendResult {
	if h.templates == nil || user == nil {
		return sendResult{}
	}
	tpl, err := h.templates.Get(slug)
	if err != nil || !tpl.Enabled || !tpl.AutoSend {
		return sendResult{}
	}
	return h.sendTemplateEmail(slug, user, tempPassword, adminID, adminEmail, ip, true)
}

func (h *Handlers) sendTemplateEmail(slug string, user *domain.User, tempPassword string, adminID int, adminEmail, ip string, requireEnabled bool) sendResult {
	return h.sendTemplateEmailWithVars(slug, user, tempPassword, nil, adminID, adminEmail, ip, requireEnabled)
}

// sendTemplateEmailWithVars adds template variables that only one slug needs,
// such as the reset link, without widening buildTemplateVars for everyone.
//
// Rendering and delivery live in mailtpl so the public reset flow can reuse
// them; the audit entries stay here because only this package knows which
// administrator is acting.
func (h *Handlers) sendTemplateEmailWithVars(slug string, user *domain.User, tempPassword string, extra notify.TemplateVars, adminID int, adminEmail, ip string, requireEnabled bool) sendResult {
	if h.mailer == nil || h.templates == nil || user == nil {
		return sendResult{Err: fmt.Errorf("email service unavailable")}
	}
	tpl, err := h.templates.Get(slug)
	if err != nil {
		return sendResult{Err: err}
	}

	pwd := strings.TrimSpace(tempPassword)
	if pwd == "" && strings.Contains(tpl.BodyHTML+tpl.Subject, "{{temporary_password}}") {
		pwd = passwordFallback
	}

	vars := h.buildTemplateVars(user, pwd)
	for k, v := range extra {
		vars[k] = v
	}

	adminPtr := &adminID
	if adminID == 0 {
		adminPtr = nil
	}
	res := h.mailer.Send(slug, user.Email, vars, adminPtr, requireEnabled)
	if res.Err != nil {
		h.logAudit(adminID, adminEmail, "EMAIL_SEND_FAILED", "EMAIL", slug, ip,
			fmt.Sprintf("to=%s template=%s", user.Email, slug))
		return sendResult{Err: res.Err}
	}
	h.logAudit(adminID, adminEmail, "EMAIL_SENT", "EMAIL", slug, ip,
		fmt.Sprintf("to=%s template=%s", user.Email, slug))
	return sendResult{Sent: res.Sent}
}

func (h *Handlers) buildTemplateVars(user *domain.User, tempPassword string) notify.TemplateVars {
	vars := mailtpl.BaseUserVars(user)
	vars["temporary_password"] = tempPassword
	// Declared even when empty: notify.Render leaves unknown placeholders
	// verbatim, so an unset device field would print {{gateway_ip}} to the user.
	vars["gateway_name"] = ""
	vars["gateway_ip"] = ""
	vars["armoire_label"] = ""
	vars["armoire_ip"] = ""
	h.enrichDeviceVars(user, vars)
	return vars
}

func (h *Handlers) enrichDeviceVars(user *domain.User, vars notify.TemplateVars) {
	if h.inventory == nil {
		return
	}
	if user.LinkedGatewayID != nil && *user.LinkedGatewayID != "" {
		gateways, err := h.inventory.GetGateways()
		if err == nil {
			for _, g := range gateways {
				if g.Hostname == *user.LinkedGatewayID {
					vars["gateway_name"] = g.Hostname
					vars["gateway_ip"] = g.IP
					break
				}
			}
			if vars["gateway_name"] == "" {
				vars["gateway_name"] = *user.LinkedGatewayID
			}
		}
	}
	if user.LinkedArmoireID != nil {
		machines, err := h.inventory.GetMachines()
		if err == nil {
			for _, m := range machines {
				if m.ID == *user.LinkedArmoireID {
					vars["armoire_label"] = m.NoSerie
					vars["armoire_ip"] = m.IP
					break
				}
			}
		}
	}
}
