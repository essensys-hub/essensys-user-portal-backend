package notify

import "testing"

func TestRenderPlaceholders(t *testing.T) {
	vars := TemplateVars{
		"first_name": "Marie",
		"portal_url": "https://mon.essensys.fr/",
	}
	got := Render("Bonjour {{first_name}} — {{portal_url}}", vars)
	want := "Bonjour Marie — https://mon.essensys.fr/"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderUnknownPlaceholderLeftEmpty(t *testing.T) {
	got := Render("{{unknown_key}}", TemplateVars{})
	if got != "" {
		t.Fatalf("expected empty replacement, got %q", got)
	}
}
