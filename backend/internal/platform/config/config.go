// Package config loads process configuration from environment variables (12-factor).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full process configuration. Secrets (DB password, API keys)
// arrive via environment from the secret manager, never from files in the repo.
type Config struct {
	Env         string // dev | staging | prod
	HTTPAddr    string
	MetricsAddr string
	DatabaseURL string
	RedisAddr   string
	S3Endpoint  string
	S3Bucket    string

	Auth0Domain   string // e.g. reliabilix.eu.auth0.com
	Auth0Audience string
	ClaimNS       string // namespace of custom tenant_id/role claims

	CORSOrigins []string
	RatePerSec  float64
	RateBurst   int

	ElectricityMapsKey string
	OTLPEndpoint       string
}

// Load reads configuration and validates what must exist outside dev.
func Load() (Config, error) {
	c := Config{
		Env:           String("ENV", "prod"),
		HTTPAddr:      String("HTTP_ADDR", ":8080"),
		MetricsAddr:   String("METRICS_ADDR", "127.0.0.1:9090"),
		DatabaseURL:   String("DATABASE_URL", ""),
		RedisAddr:     String("REDIS_ADDR", "localhost:6379"),
		S3Endpoint:    String("S3_ENDPOINT", ""),
		S3Bucket:      String("S3_BUCKET", "greenops"),
		Auth0Domain:   String("AUTH0_DOMAIN", ""),
		Auth0Audience: String("AUTH0_AUDIENCE", ""),
		ClaimNS:       String("AUTH_CLAIM_NS", "https://reliabilix.com/"),
		RatePerSec:    float64(Int("RATE_PER_SEC", 20)),
		RateBurst:     Int("RATE_BURST", 40),

		ElectricityMapsKey: String("ELECTRICITYMAPS_API_KEY", ""),
		OTLPEndpoint:       String("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}
	if v := String("CORS_ORIGINS", ""); v != "" {
		c.CORSOrigins = strings.Split(v, ",")
	}
	if c.Env == "dev" && c.DatabaseURL == "" {
		c.DatabaseURL = "postgres://greenops_api:dev-only@localhost:5432/greenops?sslmode=disable"
	}
	if c.Env != "dev" {
		for k, v := range map[string]string{"DATABASE_URL": c.DatabaseURL, "AUTH0_DOMAIN": c.Auth0Domain, "AUTH0_AUDIENCE": c.Auth0Audience} {
			if v == "" {
				return c, fmt.Errorf("config: %s is required when ENV=%s", k, c.Env)
			}
		}
	}
	return c, nil
}

func (c Config) IsDev() bool { return c.Env == "dev" }

// String returns the env var or def when unset/empty.
func String(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Int returns the env var parsed as int or def.
func Int(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// Duration returns the env var parsed as a duration or def.
func Duration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
