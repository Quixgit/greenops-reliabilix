// Package automation turns approved recommendations into reviewed change plans (phase 3). It plans and records
// decisions; it never executes changes in a customer's cloud (ADR-0017).
package automation

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/automation/application"
	ahttp "github.com/quixgit/greenops-reliabilix/backend/internal/automation/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/automation/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

// Deps are supplied by the composition root.
type Deps struct {
	Pool *pgxpool.Pool
	Recs application.Recommendations
	// Notify announces approved plans and reported results; optional.
	Notify notify.Notifier
}

type Module struct{ h ahttp.Handlers }

func New(d Deps) *Module {
	return &Module{h: ahttp.Handlers{Svc: application.Service{Repo: repository.Postgres{Pool: d.Pool}, Recs: d.Recs, Notify: d.Notify}}}
}

func (*Module) Name() string          { return "automation" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }
