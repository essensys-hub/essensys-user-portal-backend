package config

import "testing"

func TestTurnstileEnforcedProductionIgnoresDisable(t *testing.T) {
	cfg := Config{Environment: "production", TurnstileDisabled: true}
	if !cfg.TurnstileEnforced() {
		t.Fatal("production must enforce Turnstile")
	}
	if !cfg.IsProduction() {
		t.Fatal("expected production")
	}
}

func TestTurnstileDisabledNonProd(t *testing.T) {
	cfg := Config{Environment: "development", TurnstileDisabled: true}
	if cfg.TurnstileEnforced() {
		t.Fatal("non-prod disable should skip enforcement")
	}
}

func TestValidateRejectsDisabledTurnstileInProduction(t *testing.T) {
	cfg := Config{
		JWTSecret:         "0123456789abcdef",
		AdminToken:        "0123456789abcdef",
		Environment:       "production",
		ConsolidatedMode:  true,
		TurnstileDisabled: true,
		TurnstileSecretKey: "0123456789abcdef",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for TURNSTILE_DISABLED in production")
	}
}

func TestValidateRequiresTurnstileSecretInProduction(t *testing.T) {
	cfg := Config{
		JWTSecret:        "0123456789abcdef",
		AdminToken:       "0123456789abcdef",
		Environment:      "production",
		ConsolidatedMode: true,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing TURNSTILE_SECRET_KEY")
	}
}
