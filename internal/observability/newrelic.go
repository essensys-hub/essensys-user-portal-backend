package observability

import (
	"log"
	"os"
	"strings"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// InitNewRelic returns a configured agent when NEW_RELIC_ENABLED=true, otherwise nil.
func InitNewRelic() *newrelic.Application {
	if !Enabled() {
		return nil
	}

	license := os.Getenv("NEW_RELIC_LICENSE_KEY")
	if license == "" {
		log.Println("WARNING: NEW_RELIC_ENABLED=true but NEW_RELIC_LICENSE_KEY is empty")
		return nil
	}

	appName := os.Getenv("NEW_RELIC_APP_NAME")
	if appName == "" {
		appName = "essensys-user-portal-backend"
	}

	cfgOpts := []newrelic.ConfigOption{
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(license),
		newrelic.ConfigDistributedTracerEnabled(distributedTracingEnabled()),
	}

	app, err := newrelic.NewApplication(cfgOpts...)
	if err != nil {
		log.Printf("WARNING: New Relic init failed: %v", err)
		return nil
	}

	log.Printf("New Relic APM enabled for app %q", appName)
	return app
}

// Enabled reports whether New Relic instrumentation is active.
func Enabled() bool {
	return strings.EqualFold(os.Getenv("NEW_RELIC_ENABLED"), "true")
}

func distributedTracingEnabled() bool {
	v := os.Getenv("NEW_RELIC_DISTRIBUTED_TRACING_ENABLED")
	if v == "" {
		return true
	}
	return strings.EqualFold(v, "true")
}
