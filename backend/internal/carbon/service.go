// Package carbon is the carbon accounting domain module (energy, CO2e, SCI).
package carbon

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	chttp "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

type Module struct {
	h        chttp.Handlers
	repo     repository.Postgres
	provider domain.CarbonDataProvider
	log      *slog.Logger
}

// New wires the module. provider may be nil: /carbon/calculate then fails closed.
func New(pool *pgxpool.Pool, provider domain.CarbonDataProvider, log *slog.Logger) *Module {
	repo := repository.Postgres{Pool: pool}
	svc := &application.Service{Provider: provider}
	return &Module{h: chttp.Handlers{Svc: svc, Repo: repo}, repo: repo, provider: provider, log: log}
}

func (*Module) Name() string          { return "carbon" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskRefreshGrid, m.refreshGrid)
}

// refreshGrid persists the latest grid intensity of every supported region, so
// dashboards read local data and never call the provider per request.
func (m *Module) refreshGrid(ctx context.Context, _ *asynq.Task) error {
	lister, ok := m.provider.(domain.RegionLister)
	if !ok {
		m.log.Warn("grid refresh skipped: no provider configured")
		return nil
	}
	var firstErr error
	for _, region := range lister.Regions() {
		in, err := m.provider.GetIntensity(ctx, region)
		if err == nil {
			err = m.repo.SaveGridIntensity(ctx, domain.ProviderElectricityMaps, region, in.At, in.GPerKWh, false)
		}
		if err != nil {
			m.log.Error("grid refresh", "region", region, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
