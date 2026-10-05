// Package projects is the project domain module: projects, SCI functional units, policies.
package projects

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	phttp "github.com/quixgit/greenops-reliabilix/backend/internal/projects/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/repository"
)

type Module struct {
	h   phttp.Handlers
	Svc application.Service // used by the composition root (job fan-out, SCI denominator)
}

func New(pool *pgxpool.Pool) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: pool}}
	return &Module{h: phttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string          { return "projects" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
