// Command worker processes Asynq background jobs (sync, calculations, reports).
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts"
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
	pool, err := database.Connect(ctx, cfg.DatabaseURL) // DSN uses the greenops_worker role
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	var grid domain.CarbonDataProvider
	switch {
	case cfg.ElectricityMapsKey != "":
		grid = electricitymaps.New(cfg.ElectricityMapsKey)
	case cfg.IsDev():
		grid = electricitymaps.Static{GPerKWh: 400}
	}

	mux := asynq.NewServeMux()
	for _, m := range []queue.JobRegistrar{cloudaccounts.New(pool, log), carbon.New(pool, grid, log)} {
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

	srv := queue.NewServer(cfg.RedisAddr, config.Int("WORKER_CONCURRENCY", 10))
	go func() { <-ctx.Done(); srv.Shutdown() }()
	if err := srv.Run(mux); err != nil {
		log.Error("worker", "err", err)
		os.Exit(1)
	}
}
