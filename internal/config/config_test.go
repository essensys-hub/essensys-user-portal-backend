package config

import "testing"

func TestLoadConsolidatedMode(t *testing.T) {
	t.Setenv("CONSOLIDATED_MODE", "true")
	t.Setenv("EXCHANGE_STALE_TTL_SECONDS", "90")
	cfg := Load()
	if !cfg.ConsolidatedMode {
		t.Fatal("expected consolidated mode true")
	}
	if cfg.ExchangeStaleTTL.Seconds() != 90 {
		t.Fatalf("expected 90s stale ttl, got %v", cfg.ExchangeStaleTTL)
	}
}

func prodConsolidated() Config {
	return Config{
		ConsolidatedMode:    true,
		Environment:         "production",
		JWTSecret:           "jwt-secret-jwt-secret-0123",
		AdminToken:          "admin-token-admin-token-0123",
		TurnstileSecretKey:  "0x4AAAAtest",
		SupportPseudonymKey: "pseudonym-key-pseudonym-key-0123",
	}
}

func TestValidate_SupportPseudonymKeyRequiredInProduction(t *testing.T) {
	c := prodConsolidated()
	c.SupportPseudonymKey = ""
	if err := c.Validate(); err == nil {
		t.Fatal("missing SUPPORT_PSEUDONYM_KEY must be refused in production")
	}
	c.SupportPseudonymKey = "short"
	if err := c.Validate(); err == nil {
		t.Fatal("short SUPPORT_PSEUDONYM_KEY must be refused")
	}
}

// The GitHub App is optional: reports are kept pending until it exists.
func TestValidate_GitHubAppOptional(t *testing.T) {
	c := prodConsolidated()
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.GitHubAppConfigured() {
		t.Fatal("app reported configured without settings")
	}
	c.GitHubAppID, c.GitHubInstallation, c.GitHubAppKeyFile = "1", "2", "/k.pem"
	if !c.GitHubAppConfigured() {
		t.Fatal("app should be configured")
	}
}

func TestLoad_SupportDefaults(t *testing.T) {
	cfg := Load()
	if cfg.SupportPublicRepo != "essensys-hub/essensys-support-site" || cfg.SupportPrivateRepo != "essensys-hub/essensys-support" || cfg.GitHubAPIURL != "https://api.github.com" {
		t.Fatalf("defaults %+v", cfg)
	}
}
