// Package finops is the cost-side domain module: spend summary, budgets, anomalies and the reference rates that let
// cost in different currencies be reported in one.
package finops

import (
	"context"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
	fhttp "github.com/quixgit/greenops-reliabilix/backend/internal/finops/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/fxrates"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

type Module struct {
	h fhttp.Handlers
	// Svc is exported for the composition root, which adapts it to the ports of other modules.
	Svc application.Service
}

func New(pool *pgxpool.Pool) *Module {
	svc := application.Service{Repo: repository.Postgres{Pool: pool}}
	return &Module{h: fhttp.Handlers{Svc: svc}, Svc: svc}
}

func (*Module) Name() string          { return "finops" }
func (m *Module) Routes(r chi.Router) { m.h.Routes(r) }

// RegisterJobs implements queue.JobRegistrar.
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	mux.HandleFunc(queue.TaskRefreshFX, func(ctx context.Context, _ *asynq.Task) error {
		_, err := m.Svc.RefreshRates(ctx, ecbSource{c: fxrates.New()})
		return err
	})
}

// ecbSource adapts the ECB client to the application port.
type ecbSource struct{ c *fxrates.Client }

func (e ecbSource) Fetch(ctx context.Context, since time.Time) ([]domain.FXRate, error) {
	rs, err := e.c.Fetch(ctx, since)
	out := make([]domain.FXRate, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.FXRate{Day: r.Day, Currency: r.Currency, PerEUR: r.PerEUR})
	}
	return out, err
}
