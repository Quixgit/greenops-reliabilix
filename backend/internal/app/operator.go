package app

import (
	"context"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
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
func buildOperator(cfg config.Config, pool *pgxpool.Pool, st domain.SettingsPort, ints *integrations.Store, q queue.Enqueuer, mail notify.Mailer, store storage.Store, explain func(ctx context.Context, region string, day time.Time) (any, error)) *operator.Module {
	insp := queue.NewInspector(cfg.RedisAddr)
	port := integrationsPort{store: ints, allowPlain: cfg.IsDev() && cfg.SMTPAllowPlain}
	return operator.New(operator.Deps{Pool: pool, System: func() domain.System { return systemInfo(cfg, ints) }, Settings: st,
		Integrations: port, Diagnostics: diagnostics(cfg, pool, ints, port, insp, store),
		Sources: func(ctx context.Context) []domain.DataSource { return dataSources(ctx, cfg, ints) },
		Jobs:    insp, Queue: q, Mail: mail, Explain: explain})
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
			Terms:    "Free with an API key. Ember states CC-BY 4.0 for its data; some libraries cite CC BY-SA 4.0. Credit Ember where figures are shown and check its terms before redistributing derived data.",
			Note:     "A coarse second source behind Electricity Maps. It fills days where no hourly reading exists and is labelled “monthly average”; it never replaces hourly readings.", IntegrationID: "ember"},
		{ID: "eia", Name: "EIA (US generation mix, computed)", Category: "grid_intensity", Access: domain.AccessFree, State: state(ints != nil && ints.Value(ctx, "eia", "api_key") != ""),
			Metrics:  []string{"Hourly carbon intensity computed from net generation by fuel (gCO2e/kWh)"},
			Coverage: "United States balancing authorities that map to a cloud region's zone (PJM, MISO, CISO, PACW, PACE, ERCO, SC ...)",
			Terms:    "Free with an API key; U.S. government data, public domain.",
			Note:     "Computed, not measured: generic life-cycle factors per fuel applied to EIA-930 generation; imports between balancing authorities are not accounted for. Used behind a source that publishes a measured intensity.", IntegrationID: "eia"},
		{ID: "energy-charts", Name: "Energy-Charts (Fraunhofer ISE, computed)", Category: "grid_intensity", Access: domain.AccessFree, State: "ready",
			Metrics:  []string{"Hourly carbon intensity computed from generation by production type (gCO2e/kWh), with history"},
			Coverage: "European countries whose zone maps to a cloud region (DE, FR, ES, IT, PL, SE, FI, BE, NL, CH, AT, NO, DK, PT, CZ, HU, GR, IE)",
			Terms:    "Free, no key. Data of the transmission system operators published by Fraunhofer ISE under CC BY 4.0: credit Energy-Charts / Fraunhofer ISE where figures are shown.",
			Note:     "Computed, not measured: generic life-cycle factors per type applied to the public generation; cross-border flows are not accounted for and pumped storage is left out. It fills the past of European regions without a paid plan.", IntegrationID: "energy_charts"},
		{ID: "ieso", Name: "IESO (Ontario, computed)", Category: "grid_intensity", Access: domain.AccessFree, State: "ready",
			Metrics:  []string{"Hourly carbon intensity computed from generation by fuel (gCO2e/kWh), with history from 2015"},
			Coverage: "Ontario (zone CA-ON): Azure canadacentral and the cloud regions the zone map places there",
			Terms:    "Free, no key. Public reports of the Independent Electricity System Operator.",
			Note:     "Computed, not measured: generic life-cycle factors per fuel applied to Ontario's generation; imports are not accounted for. Used behind a source that publishes a measured intensity.", IntegrationID: "ieso"},
		{ID: "hydroquebec", Name: "Hydro-Québec (Quebec, computed)", Category: "grid_intensity", Access: domain.AccessFree, State: "ready",
			Metrics:  []string{"Hourly carbon intensity computed from generation by source (gCO2e/kWh)"},
			Coverage: "Quebec (zone CA-QC): Azure canadaeast, AWS ca-central-1, GCP northamerica-northeast1",
			Terms:    "Free, no key. Open data of Hydro-Québec: credit Hydro-Québec where figures are shown.",
			Note:     "Computed, not measured: generic life-cycle factors per source applied to Quebec's generation; imports are not accounted for. The open history ends on 2025-12-31, so later days are filled from the live feed (the last two days) as it is collected.", IntegrationID: "hydroquebec"},
		{ID: "entsoe", Name: "ENTSO-E Transparency Platform (Europe, computed)", Category: "grid_intensity", Access: domain.AccessFree, State: state(ints != nil && ints.Value(ctx, "entsoe", "api_token") != ""),
			Metrics:  []string{"Hourly carbon intensity computed from actual generation per production type (gCO2e/kWh), with history"},
			Coverage: "European bidding zones and countries whose zone maps to a cloud region (DE, FR, ES, IT, PL, SE, FI, BE, NL, CH, AT, NO, DK, PT, CZ, HU, GR, IE, RO, BG, HR, SK, SI, RS, LT, LV, EE); not Great Britain",
			Terms:    "Free with a personal security token; ENTSO-E's data is re-usable with credit to ENTSO-E (check its terms of use before redistributing derived data).",
			Note:     "Computed, not measured: generic life-cycle factors per type applied to the actual generation; cross-border flows are not accounted for and pumped-storage generation is left out. Without a token the source is skipped.", IntegrationID: "entsoe"},
		{ID: "ons", Name: "ONS (Brazil, computed)", Category: "grid_intensity", Access: domain.AccessFree, State: "ready",
			Metrics:  []string{"Hourly carbon intensity computed from hydro, thermal, wind and solar generation by subsystem (gCO2e/kWh), with history"},
			Coverage: "Brazil's subsystems: Southeast/Centre-West (São Paulo regions), South, Northeast and North, by the zone map (BR-CS, BR-S, BR-NE, BR-N)",
			Terms:    "Free, no key. Open data of the Operador Nacional do Sistema Elétrico under CC BY.",
			Note:     "Computed, not measured: the dataset does not split thermal generation by fuel, so one assumed factor stands for Brazil's thermal mix; distributed generation and imports are not accounted for. Used behind a source that publishes a measured intensity.", IntegrationID: "ons"},
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
			Note: "Planned: the client is written and tested (ADR-0049) but not connected to ingestion. Credentials can already be stored here. Marginal and average intensity are different measures and will be stored and shown separately.", IntegrationID: "watttime"},
		{ID: "uk-carbon-intensity", Name: "UK Carbon Intensity API", Category: "grid_intensity", Access: domain.AccessFree, State: "ready",
			Metrics: []string{"Great Britain grid intensity (gCO2e/kWh), half-hourly, measured"}, Coverage: "Great Britain (cloud regions whose zone is GB)",
			Terms: "Free, no key (National Grid ESO, CC-BY 4.0).", Note: "Used for GB regions when it is in the source priority; it needs no setup.", IntegrationID: "uk_carbon_intensity"},
		{ID: "climate-trace", Name: "Climate TRACE", Category: "inventory", Access: domain.AccessFree, State: state(ints != nil && ints.Value(ctx, "ember", "api_key") != ""),
			Metrics:  []string{"Annual power-sector emissions by country (tonnes CO2e)", "Direct-emission intensity per country and year: emissions ÷ Ember generation (gCO2e/kWh)"},
			Coverage: "Countries of the cloud regions in the catalog, the last three completed years",
			Terms:    "Free, no key; Climate TRACE data is CC BY 4.0. The generation behind the intensity comes from Ember (its key).",
			Note:     "A reference, never a calculation input: an inventory has no generation, so the platform divides it by Ember's annual generation. The figure is direct emissions (no life-cycle share), so it sits below the hourly sources' life-cycle values; it is shown on Aggregation → Source divergence to hold them against.", IntegrationID: "climatetrace"},
		{ID: "azure-billing", Name: "Microsoft Azure (Cost Management export in Blob Storage)", Category: "billing", Access: domain.AccessCustomer, State: "not_configured",
			Metrics:  []string{"Cost of every service (Cost Management API)", "Virtual machine vCPU hours and Blob storage capacity (export only)"},
			Coverage: "Subscriptions whose service principal (Cost Management Reader) the customer connected, or whose Cost Management export they shared",
			Terms:    "Customer data; read-only, cost data only. The service principal's secret is stored sealed.", Note: platformAzureNote(cfg), IntegrationID: "platform_azure"},
	}
}

func platformGCPNote(cfg config.Config) string {
	if cfg.PlatformGCPProject == "" {
		return "The platform Google Cloud project is not set; the connector is off."
	}
	return ""
}

func platformAzureNote(cfg config.Config) string {
	if cfg.PlatformAzureClientID == "" || cfg.PlatformAzureClientSecret == "" {
		return "Service-principal connections work without anything here. The platform's Microsoft Entra application is only needed to read a customer's Cost Management export from Blob Storage; it is not set."
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
