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
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/version"
)

// NewRouter builds the API. ready reports dependency health for /readyz.
func NewRouter(cfg config.Config, log *slog.Logger, v auth.Verifier, res auth.Resolver, ready func(context.Context) error, modules ...httpx.Module) http.Handler {
	r := chi.NewRouter()
	r.Use(
		httpx.WithRequestID(),
		httpx.Recover(log),
		httpx.SecureHeaders(),
		httpx.CORS(cfg.CORSOrigins),
		httpx.RateLimit(cfg.RatePerSec, cfg.RateBurst),
		// 1 MiB for everything but the cost-file upload (a vendor export is a file: 8 MiB plus the form around it)
		httpx.MaxBodyFor(1<<20, map[string]int64{"/api/v1/imports/cost": 8<<20 + 64<<10}),
		httpx.AccessLog(log),
	)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/version", func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, version.Current())
	})
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
		// Public routes: no identity (the sign-in page needs them before anyone is signed in).
		r.Group(func(r chi.Router) {
			for _, m := range modules {
				if pm, ok := m.(interface{ PublicRoutes(chi.Router) }); ok {
					pm.PublicRoutes(r)
				}
			}
		})
		// Identity-only routes: a valid token is enough (onboarding, accepting an invitation).
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthenticateIdentity(v))
			for _, m := range modules {
				if im, ok := m.(httpx.IdentityModule); ok {
					im.IdentityRoutes(r)
				}
			}
		})
		// Everything else requires a tenant membership (resolved from the database) or an API key.
		r.Group(func(r chi.Router) {
			r.Use(auth.Authenticate(v, res))
			for _, m := range modules {
				m.Routes(r)
			}
		})
	})
	return r
}
