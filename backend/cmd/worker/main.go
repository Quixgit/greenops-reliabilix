// Command worker processes Asynq background jobs: sync, normalization, carbon, recommendations, reports.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

func main() {
	log := observability.Logger("worker")
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

	pool, err := database.Connect(ctx, cfg.DatabaseURL) // DSN uses the greenops_worker role
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

	mux := asynq.NewServeMux()
	for _, m := range c.JobRegistrars() {
		m.RegisterJobs(mux)
	}
	mux.HandleFunc(queue.TaskEnsurePartitions, func(ctx context.Context, _ *asynq.Task) error {
		return database.EnsurePartitions(ctx, pool, time.Now().UTC())
	})

	go func() {
		if err := httpx.NewServer(cfg.MetricsAddr, observability.MetricsHandler()).ListenAndServe(); err != nil {
			log.Error("metrics server", "err", err)
		}
	}()

	srv := queue.NewServer(cfg.RedisAddr, config.Int("WORKER_CONCURRENCY", 10), func(_ context.Context, taskType string, err error) {
		log.Error("job failed", "task", taskType, "err", err)
		observability.CaptureError(err)
	})
	go func() { <-ctx.Done(); srv.Shutdown() }()
	if err := srv.Run(mux); err != nil {
		log.Error("worker", "err", err)
		os.Exit(1)
	}
}
