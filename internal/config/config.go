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
	return Config{
		ConsolidatedMode:  envBool("CONSOLIDATED_MODE", false),
		ExchangeStaleTTL:  envDurationSeconds("EXCHANGE_STALE_TTL_SECONDS", 120),
		DBHost:            env("DB_HOST", "127.0.0.1"),
		DBPort:            env("DB_PORT", "5432"),
		DBUser:            env("DB_USER", "essensys"),
		DBPassword:        env("DB_PASSWORD", ""),
		DBName:            env("DB_NAME", "essensys_db"),
		Port:              env("PORT", "8081"),
		MigrationsDir:     env("MIGRATIONS_DIR", "migrations"),
		CORSOrigin:        env("CORS_ORIGIN", "https://mon.essensys.fr"),
		JWTSecret:         env("JWT_SECRET", ""),
		AdminToken:        env("ADMIN_TOKEN", ""),
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
	return nil
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
	ConsolidatedMode bool
	ExchangeStaleTTL time.Duration
	DBHost           string
	DBPort           string
	DBUser           string
	DBPassword       string
	DBName           string
	Port             string
	MigrationsDir    string
	CORSOrigin       string
	JWTSecret        string
	AdminToken       string
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
