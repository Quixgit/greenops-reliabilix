// Package alerts keeps the organization's alerts: it evaluates budgets and connection health every hour, opens and
// resolves alerts by itself and lets people acknowledge or mute them (ADR-0028).
package alerts

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/application"
	ahttp "github.com/quixgit/greenops-reliabilix/backend/internal/alerts/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

type Deps struct {
	Pool        *pgxpool.Pool
	Budgets     application.BudgetSource
	Connections application.ConnectionSource
	Tenants     application.TenantLister
	Notify      notify.Notifier
	Log         *slog.Logger
}

type Module struct {
	h   ahttp.Handlers
	svc application.Service
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Budgets: d.Budgets, Connections: d.Connections,
		Tenants: d.Tenants, Notify: d.Notify, Log: d.Log}
	return &Module{h: ahttp.Handlers{Svc: svc}, svc: svc}
}

func (*Module) Name() string          { return "alerts" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskEvaluateAlerts, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.svc.EvaluateAll(ctx)
		return err
	})
}
