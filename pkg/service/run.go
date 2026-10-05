// Package service is the common bootstrap every microservice uses, so that
// security middleware, health endpoints and graceful shutdown are identical.
package service

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/quixgit/greenops-reliabilix/pkg/auth"
	"github.com/quixgit/greenops-reliabilix/pkg/config"
	"github.com/quixgit/greenops-reliabilix/pkg/httpx"
)

// Deps are the dependencies handed to a service's route registration.
type Deps struct {
	Log      *slog.Logger
	Verifier auth.Verifier
}

// Options customises the bootstrap.
type Options struct {
	// Public skips the Authenticate middleware (used only by health-like services).
	// The gateway and every business service leave it false.
	Public bool
}

// Run starts the service named name. register mounts business routes on mux;
// they are wrapped with request-id, recovery, security headers, body limit,
// access log and (unless Public) authentication.
func Run(name, defaultPort string, opts Options, register func(mux *http.ServeMux, d Deps)) {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", name)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	v, err := auth.VerifierFromEnv()
	if err != nil {
		log.Error("auth configuration", "err", err)
		os.Exit(1)
	}

	api := http.NewServeMux()
	register(api, Deps{Log: log, Verifier: v})

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	root.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	var business http.Handler = api
	if !opts.Public {
		business = httpx.Chain(api, auth.Authenticate(v))
	}
	root.Handle("/", business)

	h := httpx.Chain(root,
		httpx.WithRequestID(),
		httpx.Recover(log),
		httpx.SecureHeaders(),
		httpx.MaxBody(1<<20),
		httpx.AccessLog(log),
	)
	srv := httpx.NewServer(":"+config.String("PORT", defaultPort), h)
	if err := httpx.Serve(ctx, srv, log); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
