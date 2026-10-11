// Package notifications delivers platform events to the places tenants choose: e-mail, webhooks, Slack, and
// whatever channel is added next (ADR-0019). Other domains never import it: they publish through the
// platform/notify port, and internal/app wires this module in as the implementation.
//
// To add a channel: implement domain.Channel in channels/<name>, register it in internal/app. Nothing else
// changes: not the producers, not the tables, not the API.
package notifications

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	nhttp "github.com/quixgit/greenops-reliabilix/backend/internal/notifications/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/errlog"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// Deps are supplied by the composition root.
type Deps struct {
	Pool      *pgxpool.Pool
	Log       *slog.Logger
	Queue     queue.Enqueuer
	Channels  *domain.Registry
	Box       application.Sealer
	Directory application.Directory
	BaseURL   string
	Stubbed   []domain.Kind     // channels on a development stand-in (reported by the catalog)
	Errors    errlog.Recorder   // optional: deliveries that fail for good are reported to the operator error log
	Settings  settings.Reader   // optional
	Limits    entitlements.Gate // optional
}

type Module struct {
	h   nhttp.Handlers
	Svc application.Service
	log *slog.Logger
}

func New(d Deps) *Module {
	svc := application.Service{Inbox: repository.Postgres{Pool: d.Pool}, Repo: repository.Postgres{Pool: d.Pool}, Channels: d.Channels, Box: d.Box, Queue: d.Queue,
		Directory: d.Directory, BaseURL: d.BaseURL, Log: d.Log, Stubbed: d.Stubbed, Errors: d.Errors, Settings: d.Settings, Limits: d.Limits}
	return &Module{h: nhttp.Handlers{Svc: svc}, Svc: svc, log: d.Log}
}

func (*Module) Name() string          { return "notifications" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskDeliverNotification, func(ctx context.Context, t *asynq.Task) error {
		p, err := queue.Decode(t)
		if err != nil {
			return err
		}
		return m.Svc.Deliver(ctx, p.TenantID, p.RefID)
	})
	mux.HandleFunc(queue.TaskSweepNotifications, func(ctx context.Context, _ *asynq.Task) error {
		n, err := m.Svc.Sweep(ctx)
		if err == nil && n > 0 && m.log != nil {
			m.log.Info("notifications swept", "queued", n)
		}
		return err
	})
	mux.HandleFunc(queue.TaskPurgeNotifications, func(ctx context.Context, _ *asynq.Task) error {
		n, err := m.Svc.Purge(ctx)
		if err == nil && n > 0 && m.log != nil {
			m.log.Info("old notification deliveries purged", "deleted", n)
		}
		return err
	})
}
