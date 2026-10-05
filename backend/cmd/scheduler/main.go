// Command scheduler enqueues periodic jobs. It does no heavy work itself.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

func main() {
	log := observability.Logger("scheduler")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	s := asynq.NewScheduler(asynq.RedisClientOpt{Addr: cfg.RedisAddr}, nil)
	for spec, task := range map[string]string{
		"0 */6 * * *": queue.TaskSyncAll,          // fan-out: one sync job per healthy connection
		"0 2 * * *":   queue.TaskEnsurePartitions, // monthly partitions ahead of time
		"5 * * * *":   queue.TaskRefreshGrid,      // hourly grid intensity (Electricity Maps)
	} {
		if _, err := s.Register(spec, asynq.NewTask(task, []byte("{}"))); err != nil {
			log.Error("register", "task", task, "err", err)
			os.Exit(1)
		}
	}
	go func() { <-ctx.Done(); s.Shutdown() }()
	if err := s.Run(); err != nil {
		log.Error("scheduler", "err", err)
		os.Exit(1)
	}
}
