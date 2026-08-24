// Package mailtpl renders and delivers the transactional email templates stored
// in email_templates. It exists because both the admin package and the public
// identity flow need to send the same password_reset template, and the sending
// logic previously lived on admin.Handlers where identity could not reach it
// without importing the admin surface wholesale.
//
// Audit logging stays with the caller: only the caller knows whether an action
// was performed by an administrator or by an anonymous visitor.
package mailtpl

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/notify"
)

// TemplateStore is the slice of data.EmailTemplateStore this package needs,
// narrowed so tests can substitute it without a database.
type TemplateStore interface {
	Get(slug string) (*domain.EmailTemplate, error)
	LogSend(recipient, slug, status, errMsg string, adminID *int) error
}

// Result reports delivery separately from the caller's own outcome: issuing a
// reset token succeeds even when the mail does not leave the building.
type Result struct {
	Sent bool
	Err  error
}

type Sender struct {
	templates TemplateStore
}

func NewSender(templates TemplateStore) *Sender {
	return &Sender{templates: templates}
}

// Send renders slug with vars and mails it to recipient. actorID is the admin
// behind the action, or nil when the visitor triggered it themselves.
//
// requireEnabled is false only for admin previews and manual resends, where an
// operator is deliberately exercising a template that is not yet live.
func (s *Sender) Send(slug, recipient string, vars notify.TemplateVars, actorID *int, requireEnabled bool) Result {
	if s == nil || s.templates == nil {
		return Result{Err: fmt.Errorf("email service unavailable")}
	}
	if strings.TrimSpace(recipient) == "" {
		return Result{Err: fmt.Errorf("no recipient")}
	}

	tpl, err := s.templates.Get(slug)
	if err != nil {
		return Result{Err: err}
	}
	if tpl == nil {
		return Result{Err: fmt.Errorf("template %s not found", slug)}
	}
	if requireEnabled && !tpl.Enabled {
		return Result{Err: fmt.Errorf("template %s is disabled", slug)}
	}
	if !notify.Configured() {
		err := fmt.Errorf("SMTP configuration missing")
		s.logFailure(recipient, slug, err, actorID)
		return Result{Err: err}
	}

	subject := notify.Render(tpl.Subject, vars)
	body := notify.Render(tpl.BodyHTML, vars)
	if body == "" && tpl.BodyText != "" {
		body = "<pre>" + notify.Render(tpl.BodyText, vars) + "</pre>"
	}

	if err := notify.Send([]string{recipient}, subject, body); err != nil {
		s.logFailure(recipient, slug, err, actorID)
		log.Printf("[email] send %s to %s: %v", slug, recipient, err)
		return Result{Err: err}
	}
	_ = s.templates.LogSend(recipient, slug, "sent", "", actorID)
	return Result{Sent: true}
}

func (s *Sender) logFailure(recipient, slug string, err error, actorID *int) {
	_ = s.templates.LogSend(recipient, slug, "failed", err.Error(), actorID)
}

// BaseUserVars fills the placeholders any transactional template may reference.
// Callers layer their own on top: the admin flow adds temporary_password and the
// device fields, the reset flow adds reset_url and expires_in.
func BaseUserVars(user *domain.User) notify.TemplateVars {
	vars := notify.TemplateVars{
		"first_name":    user.FirstName,
		"last_name":     user.LastName,
		"email":         user.Email,
		"role":          user.Role,
		"portal_url":    PortalURL(),
		"support_email": SupportEmail(),
	}
	// Templates open with "Bonjour {{first_name}}", which reads as a bug when
	// the name was never collected.
	if user.FirstName == "" {
		vars["first_name"] = user.Email
	}
	return vars
}

// PortalURL keeps the trailing slash the stored templates were written against.
func PortalURL() string {
	if v := strings.TrimSpace(os.Getenv("FRONTEND_URL")); v != "" {
		return v
	}
	return "https://www.essensys.fr/"
}

func SupportEmail() string {
	if v := strings.TrimSpace(os.Getenv("SMTP_FROM")); v != "" {
		return v
	}
	return "support@essensys.fr"
}
