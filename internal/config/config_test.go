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
