// Package operator is the platform operator console (ADR-0020): a cross-tenant, metadata-only view for the people
// who run the platform, kept apart from tenant roles. Who is an operator is a database fact that only the migration
// owner can change.
package operator

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	ohttp "github.com/quixgit/greenops-reliabilix/backend/internal/operator/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

// Deps are supplied by the composition root. System reports build and integration facts (it is a function so the
// composition root, which knows the configuration, owns the answer).
type Deps struct {
	Pool   *pgxpool.Pool
	System func() domain.System
	// Settings is the configuration engine (ADR-0023).
	Settings domain.SettingsPort
	// Integrations is the external-service credential store (ADR-0029).
	Integrations domain.IntegrationsPort
	// Sources is the data source catalog (ADR-0032).
	Sources func(ctx context.Context) []domain.DataSource
	// Jobs is read-only queue telemetry (optional).
	Jobs domain.JobsPort
	// Queue starts background jobs such as the grid-history backfill (optional).
	Queue queue.Enqueuer
	// Mail e-mails announcements (optional: without it an announcement reaches the bell only).
	Mail notify.Mailer
}

type Module struct{ h ohttp.Handlers }

func New(d Deps) *Module {
	return &Module{h: ohttp.Handlers{Pool: d.Pool, Mail: d.Mail, Svc: application.Service{Repo: repository.Postgres{Pool: d.Pool}, System: d.System, Settings: d.Settings, Integrations: d.Integrations, Sources: d.Sources, Jobs: d.Jobs, Queue: d.Queue}}}
}

func (*Module) Name() string                  { return "operator" }
func (*Module) Routes(chi.Router)             {} // nothing behind tenant authentication
func (m *Module) IdentityRoutes(r chi.Router) { m.h.IdentityRoutes(r) }
