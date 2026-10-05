// Package projects is the project domain module.
package projects

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	phttp "github.com/quixgit/greenops-reliabilix/backend/internal/projects/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/repository"
)

type Module struct{ h phttp.Handlers }

func New(pool *pgxpool.Pool) *Module {
	return &Module{h: phttp.Handlers{Svc: application.Service{Repo: repository.Postgres{Pool: pool}}}}
}

func (*Module) Name() string          { return "projects" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
