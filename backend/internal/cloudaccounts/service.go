// Package cloudaccounts is the cloud connections domain module (AWS now; Azure/GCP in phase 2).
package cloudaccounts

import (
	"context"
	"errors"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	chttp "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
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
	Notify               notify.Notifier        // optional: announces connections that need attention
	Settings             settings.Reader        // optional
	Limits               entitlements.Gate      // optional
	PlatformAWSAccountID string
	// PlatformGCPServiceAccount is shown to GCP customers as the principal to grant dataset read access to.
	PlatformGCPServiceAccount string
	// Secrets seals customer-supplied secrets (nil = connections that need one are refused).
	Secrets application.Sealer
	// PlatformAzureAppID is shown to Azure customers as the application to grant read access to their export container.
	PlatformAzureAppID string
	BackfillDays       int
}

type Module struct {
	h   chttp.Handlers
	svc application.Service
	// Svc is exported for the composition root, which adapts it to the ports of other modules.
	Svc application.Service
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Providers: d.Providers, Ingestor: d.Ingestor, Rightsizing: d.Rightsizing, Notify: d.Notify, Settings: d.Settings, Limits: d.Limits, Queue: d.Queue,
		Secrets: d.Secrets, Log: d.Log, PlatformAWSAccountID: d.PlatformAWSAccountID, PlatformGCPServiceAccount: d.PlatformGCPServiceAccount, PlatformAzureAppID: d.PlatformAzureAppID, BackfillDays: d.BackfillDays}
	return &Module{h: chttp.Handlers{Svc: svc}, svc: svc, Svc: svc}
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
		if domain.NeedsCustomerAction(err) {
			return errors.Join(asynq.SkipRetry, err) // the customer must fix access or the export; retrying cannot help
		}
		return err
	})
}
