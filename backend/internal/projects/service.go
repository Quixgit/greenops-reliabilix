// Package projects is the project domain module: projects, SCI functional units, policies.
package projects

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	phttp "github.com/quixgit/greenops-reliabilix/backend/internal/projects/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/repository"
)

type Module struct {
	h   phttp.Handlers
	Svc application.Service // used by the composition root (job fan-out, SCI denominator)
}

// Deps are supplied by the composition root; Limits is optional.
type Deps struct {
	Pool   *pgxpool.Pool
	Limits entitlements.Gate
}

func New(d Deps) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: d.Pool}, Limits: d.Limits}
	return &Module{h: phttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string          { return "projects" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
