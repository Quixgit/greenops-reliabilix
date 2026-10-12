package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

// demoModule gives an organization sample data to look at before a cloud account is connected, and takes it back (ADR-0052). It lives in
// the composition root because it touches projects, usage, carbon and recommendations at once; the rows are written by two
// SECURITY DEFINER functions (migration 0069) that only ever touch what projects.demo_markers lists.
type demoModule struct {
	pool   *pgxpool.Pool
	q      queue.Enqueuer
	limits entitlements.Gate
}

const demoProjectName = "Sample data (demo)"

func (demoModule) Name() string { return "demo" }

func (m demoModule) Routes(r chi.Router) {
	r.With(auth.Require(auth.PermProjectRead)).Get("/demo", m.status)
	r.With(auth.Require(auth.PermProjectWrite)).Post("/demo", m.create)
	r.With(auth.Require(auth.PermProjectWrite)).Delete("/demo", m.remove)
}

func (m demoModule) active(r *http.Request, tenant string) (project *string, err error) {
	err = database.WithTenantTx(r.Context(), m.pool, tenant, func(tx pgx.Tx) error {
		var id string
		switch e := tx.QueryRow(r.Context(), `SELECT project_id::text FROM projects.demo_markers`).Scan(&id); {
		case e == nil:
			project = &id
		case !database.IsNotFound(e):
			return e
		}
		return nil
	})
	return
}

func (m demoModule) status(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	p, err := m.active(r, c.TenantID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": p != nil, "project_id": p})
}

func (m demoModule) create(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	if p, err := m.active(r, c.TenantID); err != nil {
		httpx.Internal(w, r, err)
		return
	} else if p != nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"active": true, "project_id": p})
		return
	}
	if err := m.limits.Allow(r.Context(), c.TenantID, entitlements.Projects); err != nil {
		if errors.Is(err, entitlements.ErrLimitReached) {
			httpx.WriteProblem(w, r, http.StatusPaymentRequired, "plan limit reached", err.Error())
			return
		}
		httpx.Internal(w, r, err)
		return
	}
	var id string
	if err := m.pool.QueryRow(r.Context(), `SELECT platform.demo_create($1::uuid, $2)::text`, c.TenantID, demoProjectName).Scan(&id); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	_ = database.WithTenantTx(r.Context(), m.pool, c.TenantID, func(tx pgx.Tx) error {
		return audit.Record(r.Context(), tx, "demo.created", "project:"+id, nil)
	})
	// The carbon figures and the recommendations come from the real engine: ask for them now instead of at the next scheduled run.
	_ = m.q.Enqueue(r.Context(), queue.TaskRecalculateAll, struct{}{}, asynq.MaxRetry(2))
	_ = m.q.Enqueue(r.Context(), queue.TaskRecommendAll, struct{}{}, asynq.MaxRetry(2), asynq.ProcessIn(90*time.Second))
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"active": true, "project_id": id})
}

func (m demoModule) remove(w http.ResponseWriter, r *http.Request) {
	c, _ := auth.FromContext(r.Context())
	var removed bool
	if err := m.pool.QueryRow(r.Context(), `SELECT platform.demo_remove($1::uuid)`, c.TenantID).Scan(&removed); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !removed {
		httpx.NotFound(w, r)
		return
	}
	_ = database.WithTenantTx(r.Context(), m.pool, c.TenantID, func(tx pgx.Tx) error {
		return audit.Record(r.Context(), tx, "demo.removed", "demo", nil)
	})
	w.WriteHeader(http.StatusNoContent)
}
