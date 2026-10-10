// Package lifecycle owns the data lifecycle of an organization: export, scheduled erasure and retention (ADR-0027).
package lifecycle

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/application"
	lhttp "github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

type Deps struct {
	Pool     *pgxpool.Pool
	Store    storage.Store
	Settings settings.Reader
	Log      *slog.Logger
}

type Module struct {
	h   lhttp.Handlers
	svc application.Service
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Store: d.Store, Settings: d.Settings, Log: d.Log}
	return &Module{h: lhttp.Handlers{Svc: svc}, svc: svc}
}

func (*Module) Name() string          { return "lifecycle" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskEraseDue, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.svc.EraseDue(ctx)
		return err
	})
	mux.HandleFunc(queue.TaskEnforceRetention, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.svc.EnforceRetention(ctx)
		return err
	})
}
