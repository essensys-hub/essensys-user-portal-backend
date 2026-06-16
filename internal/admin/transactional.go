package admin

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
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
	if h.templates == nil || user == nil {
		return sendResult{Err: fmt.Errorf("email service unavailable")}
	}
	tpl, err := h.templates.Get(slug)
	if err != nil {
		return sendResult{Err: err}
	}
	if requireEnabled && !tpl.Enabled {
		return sendResult{Err: fmt.Errorf("template %s is disabled", slug)}
	}
	if !notify.Configured() {
		err := fmt.Errorf("SMTP configuration missing")
		h.recordEmailFailure(user.Email, slug, err, adminID, adminEmail, ip)
		return sendResult{Err: err}
	}

	pwd := strings.TrimSpace(tempPassword)
	if pwd == "" && strings.Contains(tpl.BodyHTML+tpl.Subject, "{{temporary_password}}") {
		pwd = passwordFallback
	}

	vars := h.buildTemplateVars(user, pwd)
	subject := notify.Render(tpl.Subject, vars)
	body := notify.Render(tpl.BodyHTML, vars)
	if body == "" && tpl.BodyText != "" {
		body = "<pre>" + notify.Render(tpl.BodyText, vars) + "</pre>"
	}

	err = notify.Send([]string{user.Email}, subject, body)
	adminPtr := &adminID
	if adminID == 0 {
		adminPtr = nil
	}
	if err != nil {
		_ = h.templates.LogSend(user.Email, slug, "failed", err.Error(), adminPtr)
		h.logAudit(adminID, adminEmail, "EMAIL_SEND_FAILED", "EMAIL", slug, ip,
			fmt.Sprintf("to=%s template=%s", user.Email, slug))
		log.Printf("[email] send %s to %s: %v", slug, user.Email, err)
		return sendResult{Err: err}
	}
	_ = h.templates.LogSend(user.Email, slug, "sent", "", adminPtr)
	h.logAudit(adminID, adminEmail, "EMAIL_SENT", "EMAIL", slug, ip,
		fmt.Sprintf("to=%s template=%s", user.Email, slug))
	return sendResult{Sent: true}
}

func (h *Handlers) recordEmailFailure(recipient, slug string, err error, adminID int, adminEmail, ip string) {
	adminPtr := &adminID
	if adminID == 0 {
		adminPtr = nil
	}
	_ = h.templates.LogSend(recipient, slug, "failed", err.Error(), adminPtr)
	h.logAudit(adminID, adminEmail, "EMAIL_SEND_FAILED", "EMAIL", slug, ip,
		fmt.Sprintf("to=%s template=%s", recipient, slug))
}

func (h *Handlers) buildTemplateVars(user *domain.User, tempPassword string) notify.TemplateVars {
	portalURL := os.Getenv("FRONTEND_URL")
	if portalURL == "" {
		portalURL = "https://mon.essensys.fr/"
	}
	support := os.Getenv("SMTP_FROM")
	if support == "" {
		support = "support@essensys.fr"
	}
	vars := notify.TemplateVars{
		"first_name":         user.FirstName,
		"last_name":          user.LastName,
		"email":              user.Email,
		"role":               user.Role,
		"portal_url":         portalURL,
		"temporary_password": tempPassword,
		"support_email":      support,
		"gateway_name":       "",
		"gateway_ip":         "",
		"armoire_label":      "",
		"armoire_ip":         "",
	}
	if user.FirstName == "" {
		vars["first_name"] = user.Email
	}
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
