package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/automation"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon"
	carbondomain "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts"
	cloudapp "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	clouddomain "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/aws"
	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion"
	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/users"
)

// Container is the composition root: it builds every module once and wires their ports to each other.
type Container struct {
	Verifier auth.Verifier
	Resolver auth.Resolver

	modules []httpx.Module
	jobs    []queue.JobRegistrar
}

type options struct{ providers *clouddomain.Registry }

// Option customises Build (used by tests to inject fake cloud providers).
type Option func(*options)

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
	var grid carbondomain.CarbonDataProvider
	switch {
	case cfg.ElectricityMapsKey != "":
		overrides, err := electricitymaps.ParseOverrides(cfg.ElectricityMapsZoneOverrides)
		if err != nil {
			return nil, err
		}
		grid = electricitymaps.New(cfg.ElectricityMapsKey, overrides)
	case cfg.IsDev():
		grid = electricitymaps.Static{GPerKWh: 400}
	default:
		log.Warn("ELECTRICITYMAPS_API_KEY not set: only stored grid intensity is used")
	}
	registry := o.providers
	if registry == nil {
		awsProvider, err := aws.New(ctx)
		if err != nil {
			return nil, err
		}
		registry = clouddomain.NewRegistry(awsProvider)
	}

	tenantsM := tenants.New(pool)
	projectsM := projects.New(pool)
	usageM := usage.New(pool)
	ingestionM := ingestion.New(usageM.Svc, store, log)
	cloudM := cloudaccounts.New(cloudaccounts.Deps{Pool: pool, Log: log, Queue: q, Providers: registry,
		Ingestor: cloudapp.Ingestor(ingestionM.Svc), PlatformAWSAccountID: cfg.PlatformAWSAccountID, BackfillDays: cfg.SyncBackfillDays})
	pp := projectPorts{projects: projectsM.Svc}
	carbonM := carbon.New(carbon.Deps{Pool: pool, Log: log, Queue: q, Provider: grid, Usage: usageForCarbon{usage: usageM.Svc},
		Units: pp, Policies: pp, Projects: carbonProjects{projects: projectsM.Svc}})
	recsM := recommendations.New(recommendations.Deps{Pool: pool, Log: log, Queue: q, Projects: recsProjects{projects: projectsM.Svc}})
	reportsM := reports.New(reports.Deps{Pool: pool, Store: store, Queue: q})

	c := &Container{}
	c.modules = []httpx.Module{
		tenantsM, users.New(), projectsM, cloudM, ingestionM, usageM, carbonM, finops.New(pool),
		dashboard.New(pool), recsM, kubernetes.New(), reportsM, automation.New(),
	}
	c.jobs = []queue.JobRegistrar{cloudM, carbonM, recsM, reportsM}

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
