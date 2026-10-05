// Package tenants owns organizations, memberships, plans and the audit log view.
// TODO: membership invite/role-change use-cases (audited) and the Auth0 Action
// that injects tenant_id and role claims into tokens.
package tenants

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	thttp "github.com/quixgit/greenops-reliabilix/backend/internal/tenants/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/repository"
)

type Module struct{ h thttp.Handlers }

func New(pool *pgxpool.Pool) *Module {
	return &Module{h: thttp.Handlers{Repo: repository.Postgres{Pool: pool}}}
}

func (*Module) Name() string          { return "tenants" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
