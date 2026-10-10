package support

import (
	"fmt"
	"strings"
)

// Report kinds.
const (
	KindBug      = "bug"
	KindIncident = "incident"
)

// Fields are the form fields of a report. Closed fields (App, Mode, Impact)
// take values from fixed lists; the others are free text.
type Fields struct {
	App      string `json:"app,omitempty"`
	Version  string `json:"version,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Since    string `json:"since,omitempty"`
	Impact   string `json:"impact,omitempty"`
	Actual   string `json:"actual"`
	Expected string `json:"expected,omitempty"`
	Steps    string `json:"steps,omitempty"`
	Tried    string `json:"tried,omitempty"`
}

var appLabels = map[string]string{
	"web": "Portail web", "ios": "App iPhone", "android": "App Android", "gateway": "Gateway locale", "other": "Autre",
}
var modeLabels = map[string]string{"cloud": "Cloud (mon.essensys.fr)", "lan": "Réseau local", "unknown": "Je ne sais pas"}
var impactLabels = map[string]string{
	"module": "Un module", "local": "Toute la domotique locale", "remote": "Accès distant / portail cloud",
	"hardware": "Matériel, alimentation ou réseau", "unknown": "Je ne sais pas",
}

// neutralise makes user text safe to publish in a GitHub issue body (D6): no
// @mention notifies anyone and raw HTML is shown as text.
func neutralise(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return strings.ReplaceAll(s, "@", "@​")
}

// IssueTitle is the GitHub issue title of a report.
func IssueTitle(kind, title string) string {
	prefix := "[Bug] "
	if kind == KindIncident {
		prefix = "[Incident] "
	}
	return prefix + strings.ReplaceAll(neutralise(strings.TrimSpace(title)), "\n", " ")
}

// IssueBody renders the issue body. It only ever contains the form fields and
// the two pseudonymous references: never a name, email, account id, cabinet
// or gateway id.
func IssueBody(kind string, f Fields, userRef, reportRef string) string {
	var b strings.Builder
	b.WriteString("| Champ | Valeur |\n|---|---|\n")
	row := func(label, v string) {
		if v != "" {
			fmt.Fprintf(&b, "| %s | %s |\n", label, strings.ReplaceAll(neutralise(v), "|", "\\|"))
		}
	}
	if kind == KindBug {
		row("Application", appLabels[f.App])
		row("Version", f.Version)
		row("Connexion", modeLabels[f.Mode])
	} else {
		row("Impact", impactLabels[f.Impact])
		row("Depuis", f.Since)
		row("Connexion", modeLabels[f.Mode])
	}
	section := func(title, v string) {
		if strings.TrimSpace(v) != "" {
			fmt.Fprintf(&b, "\n### %s\n\n%s\n", title, neutralise(strings.TrimSpace(v)))
		}
	}
	section("Ce qui se passe", f.Actual)
	section("Ce qui était attendu", f.Expected)
	section("Étapes pour reproduire", f.Steps)
	section("Ce qui a déjà été essayé", f.Tried)
	fmt.Fprintf(&b, "\n---\nSignalé depuis www.essensys.fr par un utilisateur Essensys (%s, réf. %s).\n", userRef, reportRef)
	return b.String()
}
