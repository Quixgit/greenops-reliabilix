// Package recommendations turns carbon and cost findings into recommendations with a human approval workflow.
package recommendations

import (
	"context"
	"log/slog"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/awsprice"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/azureprice"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/gcpprice"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/application"
	rhttp "github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/repository"
)

type Deps struct {
	Pool     *pgxpool.Pool
	Log      *slog.Logger
	Queue    queue.Enqueuer
	Projects application.ProjectLister
	Notify   notify.Notifier
	// Settings is the organization settings engine (optional: the catalog defaults are used without it).
	Settings settings.Reader
	// GCPPricingKey returns the Cloud Billing Catalog API key (optional: without it the Google Cloud index is not built).
	GCPPricingKey func(ctx context.Context) string
}

type Module struct {
	h   rhttp.Handlers
	svc application.Service
	// Svc is exported for the composition root, which adapts it to the ports of other modules.
	Svc    application.Service
	gcpKey func(ctx context.Context) string
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Projects: d.Projects, Queue: d.Queue, Notify: d.Notify, Log: d.Log, Settings: d.Settings, ExtraFamilies: azureprice.GenericIDs()}
	return &Module{h: rhttp.Handlers{Svc: svc}, svc: svc, Svc: svc, gcpKey: d.GCPPricingKey}
}

func (*Module) Name() string          { return "recommendations" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskCalcRecommendation, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.Decode(t)
		if err != nil {
			return err
		}
		_, err = m.svc.Generate(ctx, p.TenantID, p.ProjectID)
		if err != nil {
			return err
		}
		// Measuring earlier changes rides on the same daily run; its failure must be retried like any other.
		_, err = m.svc.VerifyApplied(ctx, p.TenantID, p.ProjectID)
		return err
	})
	mux.HandleFunc(queue.TaskRefreshPrices, func(ctx context.Context, _ *asynq.Task) error {
		// One cloud failing does not hide the others' indexes; the first error is returned so the job is seen as incomplete.
		_, err := m.svc.RefreshPriceIndex(ctx, awsPriceSource{c: awsprice.New()}, awsRegions())
		for _, fam := range append(append([]azureprice.Family(nil), azureprice.Families...), azureprice.GenericFamilies()...) {
			if _, e := m.svc.RefreshPriceIndex(ctx, azurePriceSource{c: azureprice.New(), fam: fam}, azureprice.Regions); err == nil {
				err = e
			}
		}
		// AWS's managed services come from the bulk price list: large files, so a few regions at a time.
		for _, svc := range awsprice.Services {
			if _, e := m.svc.RefreshPriceIndex(ctx, awsBulkSource{b: awsprice.NewBulk(), svc: svc}, awsRegions()); err == nil {
				err = e
			}
		}
		if m.gcpKey != nil {
			if key := m.gcpKey(ctx); key != "" {
				for _, fam := range gcpprice.Families {
					if _, e := m.svc.RefreshPriceIndex(ctx, gcpPriceSource{c: gcpprice.New(key), fam: fam}, gcpRegions()); err == nil {
						err = e
					}
				}
			}
		}
		return err
	})
	mux.HandleFunc(queue.TaskRecommendAll, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.svc.RefreshAll(ctx)
		return err
	})
}

// azurePriceSource adapts the Azure Retail Prices client to the application port.
type azurePriceSource struct {
	c   *azureprice.Client
	fam azureprice.Family
}

// Provider is the family id: "azure" for virtual machines, "azure:postgresql" and "azure:redis" for the managed services.
func (a azurePriceSource) Provider() string { return a.fam.ID }
func (a azurePriceSource) Basis() string {
	if a.fam.Generic {
		return "azure_price_list_meters"
	}
	switch a.fam.ID {
	case "azure:postgresql":
		return "azure_postgresql_flexible_vcore_basket"
	case "azure:redis":
		return "azure_redis_cache_basket"
	}
	return "azure_vm_linux_basket"
}
func (a azurePriceSource) BuildIndex(ctx context.Context, regions []string) ([]application.PriceEntry, map[string]error, error) {
	idx, errs, err := a.c.BuildFamilyIndex(ctx, a.fam, regions)
	out := make([]application.PriceEntry, 0, len(idx))
	for _, i := range idx {
		out = append(out, application.PriceEntry{Region: i.Region, Idx: i.Idx, N: i.N})
	}
	return out, errs, err
}

// gcpPriceSource adapts the Cloud Billing Catalog client to the application port.
type gcpPriceSource struct {
	c   *gcpprice.Client
	fam gcpprice.Family
}

func (g gcpPriceSource) Provider() string { return g.fam.ID }
func (g gcpPriceSource) Basis() string {
	switch g.fam.ID {
	case "gcp:cloudsql":
		return "gcp_cloudsql_postgres_zonal_basket"
	case "gcp:memorystore":
		return "gcp_memorystore_redis_standard_basket"
	}
	return "gcp_n2_standard4_ondemand"
}
func (g gcpPriceSource) BuildIndex(ctx context.Context, regions []string) ([]application.PriceEntry, map[string]error, error) {
	idx, errs, err := g.c.BuildFamilyIndex(ctx, g.fam, regions)
	out := make([]application.PriceEntry, 0, len(idx))
	for _, i := range idx {
		out = append(out, application.PriceEntry{Region: i.Region, Idx: i.Idx, N: i.N})
	}
	return out, errs, err
}

// awsBulkSource adapts the AWS bulk price list (RDS, ElastiCache) to the application port.
type awsBulkSource struct {
	b   *awsprice.Bulk
	svc awsprice.Service
}

func (a awsBulkSource) Provider() string { return a.svc.ID }
func (a awsBulkSource) Basis() string {
	if a.svc.ID == "aws:elasticache" {
		return "aws_elasticache_redis_ondemand_basket"
	}
	return "aws_rds_postgresql_singleaz_basket"
}
func (a awsBulkSource) BuildIndex(ctx context.Context, regions []string) ([]application.PriceEntry, map[string]error, error) {
	idx, errs, err := a.b.BuildServiceIndex(ctx, a.svc, regions)
	out := make([]application.PriceEntry, 0, len(idx))
	for _, i := range idx {
		out = append(out, application.PriceEntry{Region: i.Region, Idx: i.Idx, N: i.N})
	}
	return out, errs, err
}

// gcpRegions are the Google Cloud regions the platform can place on the map.
func gcpRegions() []string {
	return []string{"us-central1", "us-east1", "us-east4", "us-east5", "us-south1", "us-west1", "us-west2", "us-west3", "northamerica-northeast1",
		"northamerica-northeast2", "southamerica-east1", "southamerica-west1", "europe-north1", "europe-west1", "europe-west2", "europe-west3",
		"europe-west4", "europe-west6", "europe-west8", "europe-west9", "europe-west12", "europe-southwest1", "europe-central2", "asia-east1",
		"asia-east2", "asia-northeast1", "asia-northeast2", "asia-northeast3", "asia-south1", "asia-south2", "asia-southeast1",
		"australia-southeast1", "australia-southeast2", "me-west1", "me-central1", "africa-south1"}
}

// awsPriceSource adapts the AWS price list client to the application port.
type awsPriceSource struct{ c *awsprice.Client }

func (awsPriceSource) Provider() string { return "aws" }
func (awsPriceSource) Basis() string    { return "aws_ec2_ondemand_basket" }
func (a awsPriceSource) BuildIndex(ctx context.Context, regions []string) ([]application.PriceEntry, map[string]error, error) {
	idx, errs, err := a.c.BuildIndex(ctx, regions)
	out := make([]application.PriceEntry, 0, len(idx))
	for _, i := range idx {
		out = append(out, application.PriceEntry{Region: i.Region, Idx: i.Idx, N: i.N})
	}
	return out, errs, err
}

// awsRegions are the regions the index is built for: every one the platform can place on the map.
func awsRegions() []string {
	out := make([]string, 0, len(awsprice.Locations))
	for r := range awsprice.Locations {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}
