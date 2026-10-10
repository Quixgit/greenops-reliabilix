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
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
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
func buildOperator(cfg config.Config, pool *pgxpool.Pool, st domain.SettingsPort, ints *integrations.Store, q queue.Enqueuer) *operator.Module {
	return operator.New(operator.Deps{Pool: pool, System: func() domain.System { return systemInfo(cfg, ints) }, Settings: st,
		Integrations: integrationsPort{store: ints, allowPlain: cfg.IsDev() && cfg.SMTPAllowPlain},
		Sources:      func(ctx context.Context) []domain.DataSource { return dataSources(ctx, cfg, ints) },
		Jobs:         queue.NewInspector(cfg.RedisAddr), Queue: q})
}

// dataSources is the registry of what the platform reads (ADR-0032). It is a catalog in code, because a source exists
// only when there is a connector for it; the state is read from the real configuration, and the connection counts are
// merged in by the operator service from the database. "planned" entries are listed so they are visible, never counted.
func dataSources(ctx context.Context, cfg config.Config, ints *integrations.Store) []domain.DataSource {
	emapsKey := ints != nil && ints.Value(ctx, "electricitymaps", "api_key") != ""
	state := func(ok bool) string {
		if ok {
			return "ready"
		}
		return "not_configured"
	}
	return []domain.DataSource{
		{ID: "electricitymaps", Name: "Electricity Maps", Category: "grid_intensity", Access: domain.AccessPaid, State: state(emapsKey),
			Metrics:  []string{"Grid carbon intensity (gCO2e/kWh)"},
			Coverage: "Zones of the Electricity Maps catalog; cloud regions map to zones in the carbon engine",
			Terms:    "Commercial API; the available zones and history depend on the plan. Check the licence before redistributing values.",
			Note:     "Without a key only stored readings are used. Readings are stored with their zone and time.", IntegrationID: "electricitymaps"},
		{ID: "ember", Name: "Ember", Category: "grid_intensity", Access: domain.AccessFree, State: state(ints != nil && ints.Value(ctx, "ember", "api_key") != ""),
			Metrics:  []string{"Monthly national carbon intensity (gCO2e/kWh)"},
			Coverage: "Countries (not sub-national zones); monthly, published with a delay of about two months",
			Terms:    "Free with an API key; CC-BY 4.0: credit Ember where figures are shown.",
			Note:     "A coarse second source behind Electricity Maps. It fills days where no hourly reading exists and is labelled “monthly average”; it never replaces hourly readings.", IntegrationID: "ember"},
		{ID: "aws-cost", Name: "AWS (Cost Explorer, FOCUS export)", Category: "billing", Access: domain.AccessCustomer, State: "not_configured",
			Metrics:  []string{"Cost", "Usage quantities", "Instance hours", "Storage"},
			Coverage: "Accounts customers connect with a read-only role (External ID)",
			Terms:    "Customer data under the customer's own AWS agreement; read-only, no credentials stored.", IntegrationID: "platform_aws"},
		{ID: "gcp-billing", Name: "Google Cloud (billing export in BigQuery)", Category: "billing", Access: domain.AccessCustomer, State: "not_configured",
			Metrics:  []string{"Cost", "Compute usage", "Storage"},
			Coverage: "Projects whose billing export dataset the customer granted read access to",
			Terms:    "Customer data; the platform service account only reads the dataset.", Note: platformGCPNote(cfg), IntegrationID: "platform_gcp"},
		{ID: "k8s-agent", Name: "Kubernetes agent (Kepler via Prometheus)", Category: "kubernetes", Access: domain.AccessCustomer, State: "not_configured",
			Metrics:  []string{"Workload energy (kWh)", "CPU and memory usage", "Requests"},
			Coverage: "Clusters where a customer runs the agent with an API key of the role agent",
			Terms:    "Customer-pushed measurements; attribution inside cloud energy, never added to totals."},
		{ID: "methodology", Name: "Platform methodology and coefficients", Category: "methodology", Access: domain.AccessInternal, State: "ready",
			Metrics:  []string{"Power and embodied coefficients", "PUE", "Region to zone mapping"},
			Coverage: "Versioned in code; every result carries its methodology version",
			Terms:    "Internal. A citable published dataset is not imported yet."},
		{ID: "watttime", Name: "WattTime", Category: "grid_intensity", Access: domain.AccessPaid, State: "planned",
			Metrics: []string{"Marginal operating emissions rate (MOER)"}, Coverage: "—", Terms: "Check access and licence before enabling.",
			Note: "Planned: the connector is not released. Credentials can already be stored here. Marginal and average intensity are different measures and will be stored and shown separately.", IntegrationID: "watttime"},
		{ID: "uk-carbon-intensity", Name: "UK Carbon Intensity API", Category: "grid_intensity", Access: domain.AccessFree, State: "ready",
			Metrics: []string{"Great Britain grid intensity (gCO2e/kWh), half-hourly, measured"}, Coverage: "Great Britain (cloud regions whose zone is GB)",
			Terms: "Free, no key (National Grid ESO, CC-BY 4.0).", Note: "Used for GB regions when it is in the source priority; it needs no setup.", IntegrationID: "uk_carbon_intensity"},
		{ID: "climate-trace", Name: "Climate TRACE", Category: "inventory", Access: domain.AccessFree, State: "planned",
			Metrics: []string{"Emissions inventory by sector and country (tonnes)"}, Coverage: "Global; annual totals in the public API, monthly in bulk downloads",
			Terms: "Free; CC-BY 4.0.",
			Note:  "Evaluated, not connected as a grid-intensity source: the public API (beta, no availability guarantee) returns emission totals in tonnes without electricity generation, so no gCO2e/kWh can be derived from it. It may later supply power-sector totals as context next to the intensity figures."},
		{ID: "azure-cost", Name: "Azure Cost Management", Category: "billing", Access: domain.AccessCustomer, State: "planned",
			Metrics: []string{"Cost", "Usage"}, Coverage: "—", Terms: "Customer data.", Note: "Planned, phase 2."},
	}
}

func platformGCPNote(cfg config.Config) string {
	if cfg.PlatformGCPProject == "" {
		return "The platform Google Cloud project is not set; the connector is off."
	}
	return ""
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
		pick("Payments (Stripe, Paddle, LiqPay, WayForPay)", has("stripe", "secret_key") || has("paddle", "api_key") || has("liqpay", "private_key") || has("wayforpay", "secret_key"), "", "optional: set a payment provider under Data sources; plans are then assigned by operators only"),
	}}
}
