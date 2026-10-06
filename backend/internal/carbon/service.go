// Package carbon is the carbon accounting domain module: energy, CO2e, SCI, grid intensity and the CI gate.
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

// Deps are supplied by the composition root. Provider may be nil: live lookups then fail closed.
type Deps struct {
	Pool     *pgxpool.Pool
	Log      *slog.Logger
	Queue    queue.Enqueuer
	Provider domain.CarbonDataProvider
	Usage    application.UsageSource
	Units    application.UnitsSource
	Policies application.PolicySource
	Projects application.ProjectLister
}

type Module struct {
	h   chttp.Handlers
	Svc *application.Service
}

func New(d Deps) *Module {
	svc := &application.Service{Store: repository.Postgres{Pool: d.Pool}, Usage: d.Usage, Units: d.Units, Policies: d.Policies,
		Projects: d.Projects, Provider: d.Provider, Queue: d.Queue, Log: d.Log}
	return &Module{h: chttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string          { return "carbon" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskCalculateCarbon, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.DecodeAs[queue.WindowPayload](t)
		if err != nil {
			return err
		}
		return m.Svc.RunWindowJob(ctx, p)
	})
	mux.HandleFunc(queue.TaskRecalculateAll, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.Svc.RecalculateAll(ctx)
		return err
	})
	mux.HandleFunc(queue.TaskRefreshGrid, func(ctx context.Context, _ *asynq.Task) error { return m.Svc.RefreshGrid(ctx) })
}
