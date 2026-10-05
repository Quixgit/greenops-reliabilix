// Package reports generates carbon, SCI and cost reports as CSV, JSON or PDF and stores them in object storage.
package reports

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/application"
	rhttp "github.com/quixgit/greenops-reliabilix/backend/internal/reports/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/repository"
)

type Deps struct {
	Pool  *pgxpool.Pool
	Store storage.Store
	Queue queue.Enqueuer
}

type Module struct {
	h   rhttp.Handlers
	svc application.Service
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Store: d.Store, Queue: d.Queue}
	return &Module{h: rhttp.Handlers{Svc: svc}, svc: svc}
}

func (*Module) Name() string          { return "reports" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskGenerateReport, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.Decode(t)
		if err != nil {
			return err
		}
		err = m.svc.Generate(ctx, p.TenantID, p.RefID)
		if err != nil {
			if n, ok := asynq.GetRetryCount(ctx); ok {
				if max, ok := asynq.GetMaxRetry(ctx); ok && n >= max {
					_ = m.svc.MarkFailed(ctx, p.TenantID, p.RefID) // last attempt: surface the failure to the user
				}
			}
		}
		return err
	})
}
