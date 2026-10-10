package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/email"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// integrationEnv maps a console field to the environment value it falls back to (already loaded into the config).
func integrationEnv(cfg config.Config) func(integration, key string) string {
	return func(integration, key string) string {
		switch integration + "/" + key {
		case "electricitymaps/api_key":
			return cfg.ElectricityMapsKey
		case "electricitymaps/zone_overrides":
			return cfg.ElectricityMapsZoneOverrides
		case "smtp/host":
			return cfg.SMTPHost
		case "smtp/port":
			if cfg.SMTPHost != "" && cfg.SMTPPort > 0 {
				return strconv.Itoa(cfg.SMTPPort)
			}
		case "smtp/username":
			return cfg.SMTPUsername
		case "smtp/password":
			return cfg.SMTPPassword
		case "smtp/from":
			return cfg.SMTPFrom
		case "platform_aws/account_id":
			return cfg.PlatformAWSAccountID
		case "platform_gcp/project":
			return cfg.PlatformGCPProject
		case "platform_gcp/service_account":
			return cfg.PlatformGCPServiceAccount
		case "object_storage/bucket":
			return cfg.S3Bucket
		case "object_storage/endpoint":
			return cfg.S3Endpoint
		case "object_storage/region":
			return cfg.S3Region
		case "object_storage/path_style":
			if cfg.S3PathStyle {
				return "true"
			}
		case "object_storage/access_key":
			return cfg.S3AccessKey
		case "object_storage/secret_key":
			return cfg.S3SecretKey
		case "observability/sentry_dsn":
			return cfg.SentryDSN
		case "observability/otlp_endpoint":
			return cfg.OTLPEndpoint
		}
		return ""
	}
}

// smtpFrom builds the transport from the effective values, or reports that e-mail is not configured.
func smtpFrom(ctx context.Context, ints *integrations.Store, allowPlain bool) (email.SMTP, string, bool) {
	host, from := ints.Value(ctx, "smtp", "host"), ints.Value(ctx, "smtp", "from")
	if host == "" || from == "" {
		return email.SMTP{}, "", false
	}
	port, err := strconv.Atoi(ints.ValueOrDefault(ctx, "smtp", "port"))
	if err != nil || port < 1 {
		port = 587
	}
	return email.SMTP{Host: host, Port: port, Username: ints.Value(ctx, "smtp", "username"), Password: ints.Value(ctx, "smtp", "password"),
		AllowPlain: allowPlain}, from, true
}

// integrationsPort is the operator console's view of the credential store plus the connection tests.
type integrationsPort struct {
	store      *integrations.Store
	allowPlain bool
}

func (p integrationsPort) Statuses(ctx context.Context) ([]integrations.Status, error) {
	return p.store.Statuses(ctx)
}

func (p integrationsPort) Set(ctx context.Context, actor, integration string, values map[string]*string) error {
	return p.store.Set(ctx, actor, integration, values)
}

// Test never echoes a provider's own error text: it can carry the customer-visible account or key fragments.
func (p integrationsPort) Test(ctx context.Context, integration string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	switch integration {
	case "electricitymaps":
		key := p.store.Value(ctx, "electricitymaps", "api_key")
		if key == "" {
			return "", errors.New("no API key is configured")
		}
		overrides, err := electricitymaps.ParseOverrides(p.store.Value(ctx, "electricitymaps", "zone_overrides"))
		if err != nil {
			return "", errors.New("the zone overrides are not valid")
		}
		zones, err := electricitymaps.New(key, overrides).Zones(ctx)
		if err != nil {
			return "", errors.New("Electricity Maps did not accept the key, or could not be reached")
		}
		return fmt.Sprintf("The key is accepted: %d zones are available.", len(zones)), nil
	case "smtp":
		s, _, ok := smtpFrom(ctx, p.store, p.allowPlain)
		if !ok {
			return "", errors.New("the server and the sender address are required")
		}
		if err := s.Check(ctx); err != nil {
			return "", err // the SMTP checker's messages are fixed strings
		}
		return "Connected to the mail server and signed in. No message was sent.", nil
	}
	return "", integrations.ErrUnknown
}

type valueReader interface {
	Value(ctx context.Context, integration, key string) string
}

// overlayIntegrations puts the values set in the operator console over the environment's. It is the "applied at
// start" half of ADR-0029: clients that are built once (storage, Sentry, tracing, the platform cloud accounts) read
// their settings here, before they are built.
func overlayIntegrations(ctx context.Context, cfg config.Config, r valueReader) config.Config {
	set := func(dst *string, integration, key string) {
		if v := r.Value(ctx, integration, key); v != "" {
			*dst = v
		}
	}
	set(&cfg.PlatformAWSAccountID, "platform_aws", "account_id")
	set(&cfg.PlatformGCPProject, "platform_gcp", "project")
	set(&cfg.PlatformGCPServiceAccount, "platform_gcp", "service_account")
	set(&cfg.S3Bucket, "object_storage", "bucket")
	set(&cfg.S3Endpoint, "object_storage", "endpoint")
	set(&cfg.S3Region, "object_storage", "region")
	set(&cfg.S3AccessKey, "object_storage", "access_key")
	set(&cfg.S3SecretKey, "object_storage", "secret_key")
	if v := r.Value(ctx, "object_storage", "path_style"); v != "" {
		cfg.S3PathStyle = v == "true"
	}
	set(&cfg.SentryDSN, "observability", "sentry_dsn")
	set(&cfg.OTLPEndpoint, "observability", "otlp_endpoint")
	return cfg
}

// ApplyIntegrations reads the console's values from the database and returns the configuration with them applied.
// A database that cannot be read (or has no table yet) leaves the environment's configuration as it is.
func ApplyIntegrations(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) config.Config {
	box, err := notificationsBox(cfg, log)
	if err != nil {
		log.Warn("integrations: sealing key unusable, console secrets are not applied", "err", err)
	}
	st := &integrations.Store{Pool: pool, Box: box, Env: integrationEnv(cfg)}
	if _, err := st.Statuses(ctx); err != nil {
		log.Warn("integrations: console values could not be read, using the environment", "err", err)
		return cfg
	}
	return overlayIntegrations(ctx, cfg, st)
}
