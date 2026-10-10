package app

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// Integration states shown in the operator console. "stub" is a development stand-in that is running instead of
// the real service; it is always labelled so, never passed off as the real thing.
const (
	ready   = "ready"
	stub    = "stub"
	missing = "missing"
)

// buildOperator wires the operator console. The integration list is the one place that says, for the whole
// platform, which external services are real, which are stubs and which are not set up yet.
func buildOperator(cfg config.Config, pool *pgxpool.Pool, st domain.SettingsPort, ints *integrations.Store) *operator.Module {
	return operator.New(operator.Deps{Pool: pool, System: func() domain.System { return systemInfo(cfg, ints) }, Settings: st,
		Integrations: integrationsPort{store: ints, allowPlain: cfg.IsDev() && cfg.SMTPAllowPlain}})
}

func systemInfo(cfg config.Config, ints *integrations.Store) domain.System {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	has := func(integration, key string) bool { return ints != nil && ints.Value(ctx, integration, key) != "" }
	pick := func(name string, real bool, devStub, ifMissing string) domain.Integration {
		switch {
		case real:
			return domain.Integration{Name: name, Status: ready}
		case cfg.IsDev() && devStub != "":
			return domain.Integration{Name: name, Status: stub, Note: devStub}
		default:
			return domain.Integration{Name: name, Status: missing, Note: ifMissing}
		}
	}
	return domain.System{Version: cfg.Release, Env: cfg.Env, ExpectedMigration: database.ExpectedMigration, Integrations: []domain.Integration{
		pick("Auth0 (sign-in)", cfg.Auth0Domain != "", "development sign-in without Auth0", "set AUTH0_DOMAIN and AUTH0_AUDIENCE"),
		pick("Object storage (reports)", cfg.S3Bucket != "", "local directory instead of S3", "set S3_BUCKET"),
		pick("Electricity Maps (grid carbon)", has("electricitymaps", "api_key"), "fixed 400 gCO2e/kWh", "set the API key in Integrations; only stored readings are used"),
		pick("AWS connector", cfg.PlatformAWSAccountID != "", "", "set PLATFORM_AWS_ACCOUNT_ID"),
		pick("GCP connector", cfg.PlatformGCPProject != "", "", "optional: set PLATFORM_GCP_PROJECT"),
		pick("Notifications key", cfg.NotificationsKey != "", "fixed development key", "set NOTIFICATIONS_SECRET_KEY; destinations cannot be created"),
		pick("E-mail (SMTP)", has("smtp", "host") && has("smtp", "from"), "messages are only logged", "set the mail server in Integrations; the e-mail channel is unavailable"),
		pick("Sentry (errors)", cfg.SentryDSN != "", "", "optional: set SENTRY_DSN"),
		pick("Tracing (OTLP)", cfg.OTLPEndpoint != "", "", "optional: set OTEL_EXPORTER_OTLP_ENDPOINT"),
		{Name: "Azure connector", Status: missing, Note: "planned, phase 2"},
		{Name: "WattTime (grid data)", Status: missing, Note: "planned, phase 2"},
		{Name: "Stripe (billing)", Status: missing, Note: "planned, phase 3"},
	}}
}
