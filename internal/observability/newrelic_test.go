package observability

import (
	"os"
	"testing"
)

func TestInitNewRelicDisabled(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "false")
	t.Setenv("NEW_RELIC_LICENSE_KEY", "test-key")

	if app := InitNewRelic(); app != nil {
		t.Fatal("expected nil app when NEW_RELIC_ENABLED=false")
	}
}

func TestInitNewRelicMissingLicense(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "true")
	os.Unsetenv("NEW_RELIC_LICENSE_KEY")

	if app := InitNewRelic(); app != nil {
		t.Fatal("expected nil app when license key is missing")
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv("NEW_RELIC_ENABLED", "true")
	if !Enabled() {
		t.Fatal("expected Enabled() true")
	}

	t.Setenv("NEW_RELIC_ENABLED", "false")
	if Enabled() {
		t.Fatal("expected Enabled() false")
	}
}
