// Package config loads process configuration from environment variables (12-factor).
package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config is the full process configuration. Secrets (DB password, API keys)
// arrive via environment from the secret manager, never from files in the repo.
type Config struct {
	Env             string // dev | staging | prod
	HTTPAddr        string
	MetricsAddr     string
	DatabaseURL     string
	RedisAddr       string
	S3Endpoint      string
	S3Bucket        string
	S3Region        string
	S3PathStyle     bool   // MinIO / R2
	S3AccessKey     string // local MinIO only; production uses the AWS credential chain
	S3SecretKey     string
	LocalStorageDir string // dev fallback when no S3 is configured

	SentryDSN            string
	Release              string
	PlatformAWSAccountID string // account that customers trust in their role policy
	// GCP: the platform's own service account (what customers grant read access to their billing dataset) and the
	// project in which BigQuery jobs run. Empty PlatformGCPProject = no GCP connector.
	PlatformGCPServiceAccount string
	PlatformGCPProject        string
	SyncBackfillDays          int

	Auth0Domain   string // e.g. reliabilix.eu.auth0.com
	Auth0Audience string
	ClaimNS       string // namespace of custom tenant_id/role claims

	CORSOrigins []string
	RatePerSec  float64
	RateBurst   int

	ElectricityMapsKey           string
	ElectricityMapsZoneOverrides string // region=ZONE,... (validated at startup)
	OTLPEndpoint                 string
}

// Load reads configuration and validates what must exist outside dev.
func Load() (Config, error) {
	c := Config{
		Env:             String("ENV", "prod"),
		HTTPAddr:        String("HTTP_ADDR", ":8080"),
		MetricsAddr:     String("METRICS_ADDR", "127.0.0.1:9090"),
		DatabaseURL:     String("DATABASE_URL", ""),
		RedisAddr:       String("REDIS_ADDR", "localhost:6379"),
		S3Endpoint:      String("S3_ENDPOINT", ""),
		S3Bucket:        String("S3_BUCKET", ""),
		S3Region:        String("S3_REGION", "us-east-1"),
		S3PathStyle:     String("S3_PATH_STYLE", "") == "true",
		S3AccessKey:     String("S3_ACCESS_KEY", ""),
		S3SecretKey:     String("S3_SECRET_KEY", ""),
		LocalStorageDir: String("LOCAL_STORAGE_DIR", ".data/storage"),

		SentryDSN:            String("SENTRY_DSN", ""),
		Release:              String("RELEASE", "dev"),
		PlatformAWSAccountID: String("PLATFORM_AWS_ACCOUNT_ID", ""),

		PlatformGCPServiceAccount: String("PLATFORM_GCP_SERVICE_ACCOUNT", ""),
		PlatformGCPProject:        String("PLATFORM_GCP_PROJECT", ""),
		SyncBackfillDays:          Int("SYNC_BACKFILL_DAYS", 60),
		Auth0Domain:               String("AUTH0_DOMAIN", ""),
		Auth0Audience:             String("AUTH0_AUDIENCE", ""),
		ClaimNS:                   String("AUTH_CLAIM_NS", "https://reliabilix.com/"),
		RatePerSec:                float64(Int("RATE_PER_SEC", 20)),
		RateBurst:                 Int("RATE_BURST", 40),

		ElectricityMapsKey:           String("ELECTRICITYMAPS_API_KEY", ""),
		ElectricityMapsZoneOverrides: String("ELECTRICITYMAPS_ZONE_OVERRIDES", ""),
		OTLPEndpoint:                 String("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}
	if v := String("CORS_ORIGINS", ""); v != "" {
		c.CORSOrigins = strings.Split(v, ",")
	}
	if c.PlatformGCPProject != "" && !gcpProjectRe.MatchString(c.PlatformGCPProject) {
		return c, fmt.Errorf("config: PLATFORM_GCP_PROJECT is not a valid GCP project id")
	}
	if c.PlatformGCPServiceAccount != "" && !gcpServiceAccountRe.MatchString(c.PlatformGCPServiceAccount) {
		return c, fmt.Errorf("config: PLATFORM_GCP_SERVICE_ACCOUNT must be a service account email (…@<project>.iam.gserviceaccount.com)")
	}
	if c.Env == "dev" && c.DatabaseURL == "" {
		c.DatabaseURL = "postgres://greenops_api:dev-only@localhost:5432/greenops?sslmode=disable"
	}
	if c.Env != "dev" {
		for k, v := range map[string]string{"DATABASE_URL": c.DatabaseURL, "AUTH0_DOMAIN": c.Auth0Domain, "AUTH0_AUDIENCE": c.Auth0Audience, "S3_BUCKET": c.S3Bucket} {
			if v == "" {
				return c, fmt.Errorf("config: %s is required when ENV=%s", k, c.Env)
			}
		}
	}
	return c, nil
}

var (
	gcpProjectRe        = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	gcpServiceAccountRe = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]@[a-z][a-z0-9-]{4,28}[a-z0-9]\.iam\.gserviceaccount\.com$`)
)

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
