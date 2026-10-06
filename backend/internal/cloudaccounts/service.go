// Package cloudaccounts is the cloud connections domain module (AWS now; Azure/GCP in phase 2).
package cloudaccounts

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	chttp "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

// Deps are supplied by the composition root.
type Deps struct {
	Pool                 *pgxpool.Pool
	Log                  *slog.Logger
	Queue                queue.Enqueuer
	Providers            *domain.Registry
	Ingestor             application.Ingestor
	Rightsizing          domain.RightsizingSink // optional
	PlatformAWSAccountID string
	BackfillDays         int
}

type Module struct {
	h   chttp.Handlers
	svc application.Service
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Providers: d.Providers, Ingestor: d.Ingestor, Rightsizing: d.Rightsizing, Queue: d.Queue,
		Log: d.Log, PlatformAWSAccountID: d.PlatformAWSAccountID, BackfillDays: d.BackfillDays}
	return &Module{h: chttp.Handlers{Svc: svc}, svc: svc}
}

func (*Module) Name() string          { return "cloudaccounts" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskSyncAll, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.svc.FanOut(ctx)
		return err
	})
	mux.HandleFunc(queue.TaskRightsizingAll, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.svc.FanOutRightsizing(ctx)
		return err
	})
	mux.HandleFunc(queue.TaskSyncRightsizing, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.Decode(t)
		if err != nil {
			return err
		}
		return m.svc.RunRightsizing(ctx, p.TenantID, p.RefID)
	})
	mux.HandleFunc(queue.TaskSyncAWSAccount, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.Decode(t)
		if err != nil {
			return err
		}
		err = m.svc.RunSync(ctx, p.TenantID, p.RefID)
		if errors.Is(err, domain.ErrAccessDenied) {
			return errors.Join(asynq.SkipRetry, err) // the customer must fix the role; retrying cannot help
		}
		return err
	})
}
