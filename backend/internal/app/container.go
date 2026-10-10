package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts"
	"github.com/quixgit/greenops-reliabilix/backend/internal/automation"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon"
	carbondomain "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts"
	cloudapp "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	clouddomain "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/aws"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/gcp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion"
	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes"
	"github.com/quixgit/greenops-reliabilix/backend/internal/licensing"
	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/errlog"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports"
	settingsmod "github.com/quixgit/greenops-reliabilix/backend/internal/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/users"
)

// Container is the composition root: it builds every module once and wires their ports to each other.
type Container struct {
	Verifier auth.Verifier
	Resolver auth.Resolver
	// Settings reads the configuration engine's effective values (also used by the worker's maintenance jobs).
	Settings settings.Reader

	modules []httpx.Module
	jobs    []queue.JobRegistrar
}

type options struct {
	providers *clouddomain.Registry
	errors    errlog.Recorder
}

// Option customises Build (used by tests to inject fake cloud providers).
type Option func(*options)

// WithErrors sets the recorder for errors that happen inside modules (default: none).
func WithErrors(r errlog.Recorder) Option { return func(o *options) { o.errors = r } }

// WithProviders replaces the cloud provider registry.
func WithProviders(r *clouddomain.Registry) Option { return func(o *options) { o.providers = r } }

// Build wires the platform and all domain modules.
func Build(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, q queue.Enqueuer, opts ...Option) (*Container, error) {
	var o options
	for _, f := range opts {
		f(&o)
	}
	store, err := NewStore(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	// External-service credentials come from the operator console first and the environment second (ADR-0029).
	box, err := notificationsBox(cfg, log)
	if err != nil {
		return nil, err
	}
	ints := &integrations.Store{Pool: pool, Box: box, Env: integrationEnv(cfg)}
	if cfg.ElectricityMapsZoneOverrides != "" { // a typo in the environment should stop the start, as before
		if _, err := electricitymaps.ParseOverrides(cfg.ElectricityMapsZoneOverrides); err != nil {
			return nil, err
		}
	}
	// Cost control: every request to a paid API is counted and capped per day (platform setting).
	meter := &apiusage.Meter{Pool: pool}
	var grid carbondomain.CarbonDataProvider = &electricitymaps.Dynamic{
		DevFallback: cfg.IsDev(),
		Allow:       func(ctx context.Context) error { return meter.Allow(ctx, "electricitymaps") },
		Done:        func(ctx context.Context, failed bool) { meter.Record(ctx, "electricitymaps", failed) },
		Source: func(ctx context.Context) (string, string) {
			return ints.Value(ctx, "electricitymaps", "api_key"), ints.Value(ctx, "electricitymaps", "zone_overrides")
		},
	}
	registry := o.providers
	if registry == nil {
		awsProvider, err := aws.New(ctx)
		if err != nil {
			return nil, err
		}
		providers := []clouddomain.CloudProvider{awsProvider}
		if cfg.PlatformGCPProject != "" { // the GCP connector is opt-in: it needs the platform's own Google identity
			gcpProvider, err := gcp.New(ctx, cfg.PlatformGCPProject)
			if err != nil {
				return nil, err
			}
			providers = append(providers, gcpProvider)
		}
		registry = clouddomain.NewRegistry(providers...)
	}

	settingsM := settingsmod.New(pool, log)
	cfgr := settingsM.Svc // settings.Reader for every domain
	meter.Cap = func(ctx context.Context, provider string) int {
		return int(cfgr.Int(ctx, "", settings.KeyMaxRequestsPerDay(provider)))
	}
	limits := limitsGate{pool: pool, cfg: cfgr}
	tenantsM := tenants.New(tenants.Deps{Pool: pool, Settings: cfgr, Limits: limits})
	notifyM, err := buildNotifications(cfg, log, pool, q, tenantsM.Svc, o.errors, cfgr, limits, ints, box)
	if err != nil {
		return nil, err
	}
	projectsM := projects.New(projects.Deps{Pool: pool, Limits: limits})
	usageM := usage.New(pool)
	ingestionM := ingestion.New(usageM.Svc, store, log)
	pp := projectPorts{projects: projectsM.Svc}
	carbonM := carbon.New(carbon.Deps{Pool: pool, Log: log, Queue: q, Provider: grid, Usage: usageForCarbon{usage: usageM.Svc},
		Units: pp, Policies: pp, Projects: carbonProjects{projects: projectsM.Svc}, Settings: cfgr, Monthly: &emberMonthly{ints: ints, meter: meter}})
	recsM := recommendations.New(recommendations.Deps{Pool: pool, Log: log, Queue: q, Projects: recsProjects{projects: projectsM.Svc}, Notify: notifyM.Svc})
	cloudM := cloudaccounts.New(cloudaccounts.Deps{Pool: pool, Log: log, Queue: q, Providers: registry,
		Ingestor: cloudapp.Ingestor(ingestionM.Svc), Rightsizing: rightsizingSink{recs: recsM.Svc}, Notify: notifyM.Svc, Settings: cfgr, Limits: limits, PlatformAWSAccountID: cfg.PlatformAWSAccountID, PlatformGCPServiceAccount: cfg.PlatformGCPServiceAccount, BackfillDays: cfg.SyncBackfillDays})
	finopsM := finops.New(pool)
	reportsM := reports.New(reports.Deps{Pool: pool, Store: store, Queue: q, Notify: notifyM.Svc, Settings: cfgr})

	alertsM := alerts.New(alerts.Deps{Pool: pool, Budgets: alertBudgets{finops: finopsM.Svc}, Connections: alertConnections{cloud: cloudM.Svc}, Carbon: alertCarbon{carbon: carbonM.Svc}, ProjectsCarbon: alertProjectCarbon{projects: projectsM.Svc, carbon: carbonM.Svc}, Settings: cfgr,
		Tenants: alertTenants{projects: projectsM.Svc}, Notify: notifyM.Svc, Log: log})
	lifecycleM := lifecycle.New(lifecycle.Deps{Pool: pool, Store: store, Settings: cfgr, Log: log})
	licensingM := licensing.New(licensing.Deps{Pool: pool, Settings: cfgr, Log: log, Billing: buildBilling(ints), ReturnURL: strings.TrimRight(cfg.PublicBaseURL, "/") + "/settings"})

	c := &Container{}
	c.modules = []httpx.Module{
		gatesModule{cfg: cfgr}, settingsM, tenantsM, users.New(), projectsM, cloudM, ingestionM, usageM, carbonM, finopsM,
		dashboard.New(pool), recsM, kubernetes.New(kubernetes.Deps{Pool: pool, Intensity: carbonIntensity{carbon: carbonM.Svc}}), reportsM, notifyM, buildOperator(cfg, pool, settingsM.Svc, ints, q), automation.New(automation.Deps{Pool: pool, Recs: automationRecs{recs: recsM.Svc}}), lifecycleM, licensingM, alertsM,
	}
	c.Settings = cfgr
	c.jobs = []queue.JobRegistrar{cloudM, carbonM, recsM, reportsM, notifyM, lifecycleM, licensingM, alertsM}

	switch {
	case cfg.IsDev():
		c.Verifier = auth.KeyedVerifier{Lookup: tenantsM.Svc.LookupAPIKey, Fallback: auth.DevVerifier{}}
	default:
		c.Verifier = auth.KeyedVerifier{Lookup: tenantsM.Svc.LookupAPIKey, Fallback: auth.NewOIDCVerifier(cfg.Auth0Domain, cfg.Auth0Audience, cfg.ClaimNS)}
	}
	c.Resolver = tenantsM.Svc
	return c, nil
}

func (c *Container) Modules() []httpx.Module             { return c.modules }
func (c *Container) JobRegistrars() []queue.JobRegistrar { return c.jobs }

// NewStore picks object storage: S3-compatible when configured, a local directory in development.
func NewStore(ctx context.Context, cfg config.Config, log *slog.Logger) (storage.Store, error) {
	if cfg.S3Bucket != "" {
		return storage.NewS3(ctx, storage.S3Config{Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, Region: cfg.S3Region,
			PathStyle: cfg.S3PathStyle, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey})
	}
	if !cfg.IsDev() {
		return nil, fmt.Errorf("app: S3_BUCKET is required outside ENV=dev")
	}
	if err := os.MkdirAll(cfg.LocalStorageDir, 0o750); err != nil {
		return nil, err
	}
	log.Warn("S3_BUCKET not set: using local directory storage (development only)", "dir", cfg.LocalStorageDir)
	return storage.FSStore{Root: cfg.LocalStorageDir}, nil
}
