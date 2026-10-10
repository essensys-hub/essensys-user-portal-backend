package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// minSecretLen is the minimum accepted length for security-critical secrets.
const minSecretLen = 16

// insecureSecrets are well-known placeholder values that must never reach
// production. They are rejected at startup by Validate.
var insecureSecrets = map[string]bool{
	"":                                      true,
	"insecure-dev-secret":                   true,
	"default-insecure-jwt-secret-change-me": true,
	"essensys-admin-secret":                 true,
	"changeme_random_secret":                true,
	"changeme":                              true,
	"1234567890":                            true,
}

// Load reads runtime configuration from environment variables.
func Load() Config {
	envName := env("ENV", "")
	if envName == "" {
		envName = env("APP_ENV", "")
	}
	return Config{
		ConsolidatedMode:    envBool("CONSOLIDATED_MODE", false),
		ExchangeStaleTTL:    envDurationSeconds("EXCHANGE_STALE_TTL_SECONDS", 120),
		DBHost:              env("DB_HOST", "127.0.0.1"),
		DBPort:              env("DB_PORT", "5432"),
		DBUser:              env("DB_USER", "essensys"),
		DBPassword:          env("DB_PASSWORD", ""),
		DBName:              env("DB_NAME", "essensys_db"),
		Port:                env("PORT", "8081"),
		MigrationsDir:       env("MIGRATIONS_DIR", "migrations"),
		CORSOrigin:          env("CORS_ORIGIN", "https://mon.essensys.fr"),
		JWTSecret:           env("JWT_SECRET", ""),
		AdminToken:          env("ADMIN_TOKEN", ""),
		Environment:         envName,
		TurnstileSecretKey:  env("TURNSTILE_SECRET_KEY", ""),
		TurnstileDisabled:   envBool("TURNSTILE_DISABLED", false),
		RegisterRateLimit:   envInt("REGISTER_RATE_LIMIT", 5),
		RegisterRateWindow:  envDurationSeconds("REGISTER_RATE_WINDOW_SECONDS", 3600),
		SupportPseudonymKey: env("SUPPORT_PSEUDONYM_KEY", ""),
		SupportPublicRepo:   env("SUPPORT_PUBLIC_REPO", "essensys-hub/essensys-support-site"),
		SupportPrivateRepo:  env("SUPPORT_PRIVATE_REPO", "essensys-hub/essensys-support"),
		SupportProjectID:    env("SUPPORT_PROJECT_ID", ""),
		GitHubAppID:         env("GITHUB_APP_ID", ""),
		GitHubInstallation:  env("GITHUB_APP_INSTALLATION_ID", ""),
		GitHubAppKeyFile:    env("GITHUB_APP_PRIVATE_KEY_FILE", ""),
		GitHubAPIURL:        env("GITHUB_API_URL", "https://api.github.com"),
	}
}

// Validate fails closed: it returns an error when a security-critical secret is
// missing, too short, or set to a known-insecure placeholder. Callers must abort
// startup on error so the service never runs with guessable credentials.
func (c Config) Validate() error {
	if err := checkSecret("JWT_SECRET", c.JWTSecret); err != nil {
		return err
	}
	if err := checkSecret("ADMIN_TOKEN", c.AdminToken); err != nil {
		return err
	}
	if c.IsProduction() && c.ConsolidatedMode && c.TurnstileDisabled {
		return fmt.Errorf("TURNSTILE_DISABLED is not allowed when ENV=production")
	}
	if c.IsProduction() && c.ConsolidatedMode && strings.TrimSpace(c.TurnstileSecretKey) == "" {
		return fmt.Errorf("TURNSTILE_SECRET_KEY is required when ENV=production and CONSOLIDATED_MODE=true")
	}
	// The U- reference of a support report must stay the same for an account
	// from its very first report, so the key cannot be left to a default.
	if c.IsProduction() && c.ConsolidatedMode {
		if err := checkSecret("SUPPORT_PSEUDONYM_KEY", c.SupportPseudonymKey); err != nil {
			return err
		}
	}
	return nil
}

// IsProduction reports whether the process is configured as production.
func (c Config) IsProduction() bool {
	e := strings.ToLower(strings.TrimSpace(c.Environment))
	return e == "production" || e == "prod"
}

// TurnstileEnforced is true when public register must verify Turnstile.
// Production always enforces; non-production may disable via TURNSTILE_DISABLED.
func (c Config) TurnstileEnforced() bool {
	if c.IsProduction() {
		return true
	}
	if c.TurnstileDisabled {
		return false
	}
	return true
}

func checkSecret(name, value string) error {
	if insecureSecrets[value] {
		return fmt.Errorf("%s is missing or set to a known-insecure default; set a strong unique value", name)
	}
	if len(value) < minSecretLen {
		return fmt.Errorf("%s is too short (%d chars); require at least %d", name, len(value), minSecretLen)
	}
	return nil
}

type Config struct {
	ConsolidatedMode   bool
	ExchangeStaleTTL   time.Duration
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	Port               string
	MigrationsDir      string
	CORSOrigin         string
	JWTSecret          string
	AdminToken         string
	Environment        string
	TurnstileSecretKey string
	TurnstileDisabled  bool
	RegisterRateLimit  int
	RegisterRateWindow time.Duration
	// Support reports (support-reports-2026-10-004). Without the three
	// GitHub App settings, reports are accepted and stay pending.
	SupportPseudonymKey string
	SupportPublicRepo   string
	SupportPrivateRepo  string
	SupportProjectID    string
	GitHubAppID         string
	GitHubInstallation  string
	GitHubAppKeyFile    string
	GitHubAPIURL        string
}

// GitHubAppConfigured reports whether the support module can reach GitHub.
func (c Config) GitHubAppConfigured() bool {
	return c.GitHubAppID != "" && c.GitHubInstallation != "" && c.GitHubAppKeyFile != ""
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1" || v == "yes"
}

func envDurationSeconds(key string, fallbackSec int) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(fallbackSec) * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return time.Duration(fallbackSec) * time.Second
	}
	return time.Duration(n) * time.Second
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
