// Package operator is the platform operator console (ADR-0020): a cross-tenant, metadata-only view for the people
// who run the platform, kept apart from tenant roles. Who is an operator is a database fact that only the migration
// owner can change.
package operator

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	ohttp "github.com/quixgit/greenops-reliabilix/backend/internal/operator/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/repository"
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
}

type Module struct{ h ohttp.Handlers }

func New(d Deps) *Module {
	return &Module{h: ohttp.Handlers{Svc: application.Service{Repo: repository.Postgres{Pool: d.Pool}, System: d.System, Settings: d.Settings, Integrations: d.Integrations}}}
}

func (*Module) Name() string                  { return "operator" }
func (*Module) Routes(chi.Router)             {} // nothing behind tenant authentication
func (m *Module) IdentityRoutes(r chi.Router) { m.h.IdentityRoutes(r) }
