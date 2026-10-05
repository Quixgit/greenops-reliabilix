// Command api is the HTTP API process.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

func main() {
	log := observability.Logger("api")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	flushSentry, err := observability.InitSentry(cfg.SentryDSN, cfg.Env, cfg.Release)
	if err != nil {
		log.Error("sentry", "err", err)
		os.Exit(1)
	}
	defer flushSentry()
	httpx.PanicHook = func(v any) { observability.CaptureError(panicError{v}) }

	flushTraces, err := observability.InitTracing(ctx, "api", cfg.OTLPEndpoint)
	if err != nil {
		log.Error("tracing", "err", err)
		os.Exit(1)
	}
	defer func() { _ = flushTraces(context.Background()) }()

	pool, err := database.Connect(ctx, cfg.DatabaseURL) // DSN uses the greenops_api role
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	q := queue.NewClient(cfg.RedisAddr)
	defer func() { _ = q.Close() }()

	c, err := app.Build(ctx, cfg, log, pool, q)
	if err != nil {
		log.Error("wiring", "err", err)
		os.Exit(1)
	}
	router := app.NewRouter(cfg, log, c.Verifier, c.Resolver, pool.Ping, c.Modules()...)

	go func() { // metrics on an internal-only address
		if err := httpx.NewServer(cfg.MetricsAddr, observability.MetricsHandler()).ListenAndServe(); err != nil {
			log.Error("metrics server", "err", err)
		}
	}()

	srv := httpx.NewServer(cfg.HTTPAddr, otelhttp.NewHandler(router, "api"))
	if err := httpx.Serve(ctx, srv, log); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}

type panicError struct{ v any }

func (p panicError) Error() string { return "panic recovered in HTTP handler" }
