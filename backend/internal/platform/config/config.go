// Package config loads process configuration from environment variables (12-factor).
package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
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
	// Azure: the platform's own Microsoft Entra application. Customers create its service principal in their tenant
	// and grant it read access to their export container. Empty id or secret = no Azure connector.
	PlatformAzureClientID     string
	PlatformAzureClientSecret string
	SyncBackfillDays          int

	Auth0Domain   string // e.g. reliabilix.eu.auth0.com
	Auth0Audience string
	ClaimNS       string // namespace of custom tenant_id/role claims

	CORSOrigins []string
	RatePerSec  float64
	RateBurst   int

	ElectricityMapsKey           string
	EmberKey                     string
	EnergyChartsBaseURL          string
	UKCIBaseURL                  string
	EIAKey                       string
	EntsoeToken                  string
	GCPPricingKey                string
	ElectricityMapsZoneOverrides string // region=ZONE,... (validated at startup)
	OTLPEndpoint                 string

	// Notifications (ADR-0019). Without NotificationsKey destinations cannot be created (their addresses are
	// sealed with it); without SMTP_HOST the e-mail channel is unavailable (development logs instead).
	NotificationsKey string // base64, 32 random bytes: openssl rand -base64 32
	PublicBaseURL    string // address of the web app, for links in notifications
	SMTPHost         string
	SMTPPort         int
	SMTPUsername     string
	SMTPPassword     string
	SMTPFrom         string // "Reliabilix <notifications@example.com>"
	SMTPAllowPlain   bool   // development mail catcher without TLS; ignored outside ENV=dev
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
		PlatformAzureClientID:     String("PLATFORM_AZURE_CLIENT_ID", ""),
		PlatformAzureClientSecret: String("PLATFORM_AZURE_CLIENT_SECRET", ""),
		SyncBackfillDays:          Int("SYNC_BACKFILL_DAYS", 60),
		Auth0Domain:               String("AUTH0_DOMAIN", ""),
		Auth0Audience:             String("AUTH0_AUDIENCE", ""),
		ClaimNS:                   String("AUTH_CLAIM_NS", "https://reliabilix.com/"),
		RatePerSec:                float64(Int("RATE_PER_SEC", 20)),
		RateBurst:                 Int("RATE_BURST", 40),

		ElectricityMapsKey:           String("ELECTRICITYMAPS_API_KEY", ""),
		EmberKey:                     String("EMBER_API_KEY", ""),
		EnergyChartsBaseURL:          String("ENERGYCHARTS_BASE_URL", ""),
		UKCIBaseURL:                  String("UKCI_BASE_URL", ""),
		EIAKey:                       String("EIA_API_KEY", ""),
		EntsoeToken:                  String("ENTSOE_API_TOKEN", ""),
		GCPPricingKey:                String("GCP_PRICING_API_KEY", ""),
		ElectricityMapsZoneOverrides: String("ELECTRICITYMAPS_ZONE_OVERRIDES", ""),
		OTLPEndpoint:                 String("OTEL_EXPORTER_OTLP_ENDPOINT", ""),

		NotificationsKey: String("NOTIFICATIONS_SECRET_KEY", ""),
		PublicBaseURL:    String("PUBLIC_BASE_URL", ""),
		SMTPHost:         String("SMTP_HOST", ""),
		SMTPPort:         Int("SMTP_PORT", 587),
		SMTPUsername:     String("SMTP_USERNAME", ""),
		SMTPPassword:     String("SMTP_PASSWORD", ""),
		SMTPFrom:         String("SMTP_FROM", ""),
		SMTPAllowPlain:   String("SMTP_ALLOW_PLAIN", "") == "true",
	}
	if v := String("CORS_ORIGINS", ""); v != "" {
		c.CORSOrigins = strings.Split(v, ",")
	}
	if c.PlatformGCPProject != "" && !gcpProjectRe.MatchString(c.PlatformGCPProject) {
		return c, fmt.Errorf("config: PLATFORM_GCP_PROJECT is not a valid GCP project id")
	}
	if c.PlatformAzureClientID != "" && !azureGUIDRe.MatchString(c.PlatformAzureClientID) {
		return c, fmt.Errorf("config: PLATFORM_AZURE_CLIENT_ID must be an application (client) id GUID")
	}
	if c.PlatformGCPServiceAccount != "" && !gcpServiceAccountRe.MatchString(c.PlatformGCPServiceAccount) {
		return c, fmt.Errorf("config: PLATFORM_GCP_SERVICE_ACCOUNT must be a service account email (…@<project>.iam.gserviceaccount.com)")
	}
	if err := c.validateNotifications(); err != nil {
		return c, err
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

func (c Config) validateNotifications() error {
	if c.NotificationsKey != "" {
		if _, err := secretbox.ParseKey(c.NotificationsKey); err != nil {
			return fmt.Errorf("config: NOTIFICATIONS_SECRET_KEY: %w", err)
		}
	}
	if c.PublicBaseURL != "" {
		u, err := url.Parse(c.PublicBaseURL)
		if err != nil {
			return fmt.Errorf("config: PUBLIC_BASE_URL is not a valid URL")
		}
		schemeOK := u.Scheme == "https" || (c.IsDev() && u.Scheme == "http")
		if u.Host == "" || u.User != nil || !schemeOK || (u.Path != "" && u.Path != "/") {
			return fmt.Errorf("config: PUBLIC_BASE_URL must be an https URL without a path (http is allowed with ENV=dev)")
		}
	}
	if c.SMTPHost != "" {
		if c.SMTPPort < 1 || c.SMTPPort > 65535 {
			return fmt.Errorf("config: SMTP_PORT must be 1-65535")
		}
		if _, err := mail.ParseAddress(c.SMTPFrom); err != nil {
			return fmt.Errorf("config: SMTP_FROM must be a valid sender address when SMTP_HOST is set")
		}
	}
	return nil
}

var (
	gcpProjectRe        = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	azureGUIDRe         = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
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
