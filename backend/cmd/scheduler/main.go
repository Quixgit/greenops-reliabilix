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
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/version"
)

func main() {
	log := observability.Logger("scheduler")
	log.Info("starting", version.Attrs()...)
	observability.RegisterBuildInfo()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	s := asynq.NewScheduler(asynq.RedisClientOpt{Addr: cfg.RedisAddr}, nil)
	for spec, task := range map[string]string{
		"0 */6 * * *": queue.TaskSyncAll,            // fan-out: one sync job per connection (sync -> carbon -> recommendations chain)
		"0 2 * * *":   queue.TaskEnsurePartitions,   // monthly partitions ahead of time
		"5 * * * *":   queue.TaskRefreshGrid,        // hourly grid intensity (Electricity Maps)
		"30 3 * * *":  queue.TaskRecalculateAll,     // last 35 days: picks up newly reported functional units (SCI)
		"30 4 * * *":  queue.TaskRecommendAll,       // re-evaluate after policy or grid changes
		"45 3 * * *":  queue.TaskRightsizingAll,     // provider rightsizing findings change slowly: once a day
		"* * * * *":   queue.TaskSweepNotifications, // outbox safety net: retries and lost queue messages
		"15 5 * * *":  queue.TaskPurgeNotifications, // drop finished deliveries past retention
		"25 5 * * *":  queue.TaskPurgeErrors,        // drop handled error groups past retention
		"35 5 * * *":  queue.TaskPurgeOperatorLog,   // operator access records past retention
		"20 * * * *":  queue.TaskEvaluateAlerts,     // budget and connection alerts
		"10 * * * *":  queue.TaskEraseDue,           // organizations whose erasure grace period is over
		"50 5 * * *":  queue.TaskEnforceRetention,   // usage and carbon partitions past the retention window
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
