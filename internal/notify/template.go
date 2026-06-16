package notify

import (
	"strings"
)

// TemplateVars holds allowlisted placeholder values for transactional emails.
type TemplateVars map[string]string

// SampleTemplateVars returns demo values for admin preview.
func SampleTemplateVars() TemplateVars {
	return TemplateVars{
		"first_name":          "Jean",
		"last_name":           "Dupont",
		"email":               "jean.dupont@example.com",
		"role":                "guest_local",
		"portal_url":          "https://mon.essensys.fr/",
		"temporary_password":  "MotDePasseExemple",
		"gateway_name":        "essensys-gateway",
		"gateway_ip":          "82.67.136.197",
		"armoire_label":       "ARM-001",
		"armoire_ip":          "10.0.1.42",
		"support_email":       "noreply@essensys.fr",
	}
}

// Render replaces {{key}} placeholders using only keys present in vars.
func Render(text string, vars TemplateVars) string {
	out := text
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
