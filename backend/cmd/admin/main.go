// Command admin runs maintenance jobs synchronously, without Redis. Use the worker role DSN:
//
//	DATABASE_URL=postgres://greenops_worker:...@host/greenops admin recalc
//
// Commands: doctor (checks every external dependency and says what to fix), recalc (carbon for the last 35 days of every project, then recommendations),
// recommend (recommendation analysis for every project), refresh-grid (grid intensity), partitions.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: admin doctor|recalc|recommend|refresh-grid|partitions")
		os.Exit(2)
	}
	log := observability.Logger("admin")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if os.Args[1] == "doctor" { // reports configuration problems itself instead of aborting on them
		os.Exit(runDoctor(ctx, cfg, err, log))
	}
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	inline := &queue.Inline{}
	c, err := app.Build(ctx, cfg, log, pool, inline)
	if err != nil {
		log.Error("wiring", "err", err)
		os.Exit(1)
	}
	mux := asynq.NewServeMux()
	for _, m := range c.JobRegistrars() {
		m.RegisterJobs(mux)
	}
	inline.Mux = mux

	var run func() error
	switch os.Args[1] {
	case "recalc":
		run = func() error { return mux.ProcessTask(ctx, asynq.NewTask(queue.TaskRecalculateAll, []byte("{}"))) }
	case "recommend":
		run = func() error { return mux.ProcessTask(ctx, asynq.NewTask(queue.TaskRecommendAll, []byte("{}"))) }
	case "refresh-grid":
		run = func() error { return mux.ProcessTask(ctx, asynq.NewTask(queue.TaskRefreshGrid, []byte("{}"))) }
	case "partitions":
		run = func() error { return database.EnsurePartitions(ctx, pool, time.Now().UTC()) }
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
	if err := run(); err != nil {
		log.Error("failed", "cmd", os.Args[1], "err", err)
		os.Exit(1)
	}
	log.Info("done", "cmd", os.Args[1])
}
