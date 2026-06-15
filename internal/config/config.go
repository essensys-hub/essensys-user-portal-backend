package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

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
		JWTSecret:         env("JWT_SECRET", "insecure-dev-secret"),
	}
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
