// Package tenants owns organizations, memberships, invitations, API keys and the audit log view.
package tenants

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/application"
	thttp "github.com/quixgit/greenops-reliabilix/backend/internal/tenants/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/repository"
)

type Module struct {
	h   thttp.Handlers
	Svc application.Service // Resolve / LookupAPIKey are consumed by the authentication middleware
}

func New(pool *pgxpool.Pool) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: pool}}
	return &Module{h: thttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string                  { return "tenants" }
func (m *Module) Routes(r chi.Router)         { m.h.Routes(r) }
func (m *Module) IdentityRoutes(r chi.Router) { m.h.IdentityRoutes(r) }
