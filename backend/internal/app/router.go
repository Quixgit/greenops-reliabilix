// Package app assembles the HTTP router from platform middleware and domain modules.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

// NewRouter builds the API. ready reports dependency health for /readyz.
func NewRouter(cfg config.Config, log *slog.Logger, v auth.Verifier, ready func(context.Context) error, modules ...httpx.Module) http.Handler {
	r := chi.NewRouter()
	r.Use(
		httpx.WithRequestID(),
		httpx.Recover(log),
		httpx.SecureHeaders(),
		httpx.CORS(cfg.CORSOrigins),
		httpx.RateLimit(cfg.RatePerSec, cfg.RateBurst),
		httpx.MaxBody(1<<20),
		httpx.AccessLog(log),
	)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			httpx.WriteProblem(w, req, http.StatusServiceUnavailable, "not ready", "")
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Authenticate(v))
		for _, m := range modules {
			m.Routes(r)
		}
	})
	return r
}
