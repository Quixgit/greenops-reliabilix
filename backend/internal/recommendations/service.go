// Package recommendations turns carbon and cost findings into recommendations with a human approval workflow.
package recommendations

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/application"
	rhttp "github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/repository"
)

type Deps struct {
	Pool     *pgxpool.Pool
	Log      *slog.Logger
	Queue    queue.Enqueuer
	Projects application.ProjectLister
}

type Module struct {
	h   rhttp.Handlers
	svc application.Service
	// Svc is exported for the composition root, which adapts it to the ports of other modules.
	Svc application.Service
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Projects: d.Projects, Queue: d.Queue, Log: d.Log}
	return &Module{h: rhttp.Handlers{Svc: svc}, svc: svc, Svc: svc}
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
		return err
	})
	mux.HandleFunc(queue.TaskRecommendAll, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.svc.RefreshAll(ctx)
		return err
	})
}
