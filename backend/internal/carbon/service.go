// Package carbon is the carbon accounting domain module: energy, CO2e, SCI, grid intensity and the CI gate.
package carbon

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	chttp "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
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
	// Extras are additional free sources of measured readings (optional).
	Extras []application.HourlyExtra
	// Monthly supplies monthly national figures (optional).
	Monthly application.MonthlyProvider
	// Inventory supplies annual reference intensities derived from an emissions inventory (optional).
	Inventory application.InventoryProvider
	// Settings supplies the grid source policy (optional: catalog defaults).
	Settings settings.Reader
}

type Module struct {
	h   chttp.Handlers
	Svc *application.Service
}

func New(d Deps) *Module {
	svc := &application.Service{Store: repository.Postgres{Pool: d.Pool}, Usage: d.Usage, Units: d.Units, Policies: d.Policies,
		Projects: d.Projects, Provider: d.Provider, Queue: d.Queue, Log: d.Log, Settings: d.Settings, Monthly: d.Monthly, Inventory: d.Inventory, Extras: d.Extras}
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
		err = m.Svc.RunWindowJob(ctx, p)
		if errors.Is(err, application.ErrUnknownVersion) {
			return errors.Join(asynq.SkipRetry, err) // a retry cannot make an unknown version known
		}
		return err
	})
	mux.HandleFunc(queue.TaskRecalculateAll, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.Svc.RecalculateAll(ctx)
		return err
	})
	mux.HandleFunc(queue.TaskRefreshGrid, func(ctx context.Context, _ *asynq.Task) error { return m.Svc.RefreshGrid(ctx) })
	mux.HandleFunc(queue.TaskRecalculateWindow, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.DecodeAs[queue.RecalcPayload](t)
		if err != nil {
			return err
		}
		from, e1 := time.Parse(time.DateOnly, p.From)
		to, e2 := time.Parse(time.DateOnly, p.To)
		if e1 != nil || e2 != nil || !to.After(from) || to.Sub(from) > 400*24*time.Hour {
			return errors.Join(asynq.SkipRetry, errors.New("carbon: bad recalculation window"))
		}
		n, err := m.Svc.RecalculateEverything(ctx, from, to)
		m.Svc.Log.Info("recalculation of every project queued", "projects", n, "from", p.From, "to", p.To, "err", err)
		return err
	})
	mux.HandleFunc(queue.TaskAutoLoadHistory, func(ctx context.Context, _ *asynq.Task) error {
		res, err := m.Svc.AutoLoadHistory(ctx)
		if len(res.Regions) > 0 {
			m.Svc.Log.Info("grid history loaded by itself", "regions", len(res.Regions), "projects_recalculated", res.Recalc, "err", err)
		}
		return err
	})
	mux.HandleFunc(queue.TaskRefreshMonthly, func(ctx context.Context, _ *asynq.Task) error {
		n, err := m.Svc.RefreshMonthly(ctx)
		m.Svc.Log.Info("monthly grid data refreshed", "figures", n, "err", err)
		return err
	})
	mux.HandleFunc(queue.TaskBackfillIntensity, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.DecodeAs[queue.BackfillPayload](t)
		if err != nil {
			return err
		}
		from, e1 := time.Parse(time.DateOnly, p.From)
		to, e2 := time.Parse(time.DateOnly, p.To)
		if e1 != nil || e2 != nil || !to.After(from) {
			return errors.Join(asynq.SkipRetry, errors.New("carbon: bad backfill window"))
		}
		n, err := m.Svc.BackfillIntensity(ctx, p.Regions, from, to)
		m.Svc.Log.Info("grid backfill finished", "regions", len(p.Regions), "readings", n, "err", err)
		// Loading the past also refreshes the monthly national figures; one source failing does not hide the other's error.
		if _, mErr := m.Svc.RefreshMonthly(ctx); err == nil {
			err = mErr
		}
		// A key or plan that does not return past data will not start to after five attempts: say so once and stop.
		if errors.Is(err, domain.ErrPastUnavailable) || errors.Is(err, domain.ErrNotConfigured) {
			err = errors.Join(asynq.SkipRetry, err)
		}
		return err
	})
}
