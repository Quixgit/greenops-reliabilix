// Command admin runs maintenance jobs synchronously, without Redis. Use the worker role DSN:
//
//	DATABASE_URL=postgres://greenops_worker:...@host/greenops admin recalc
//
// Commands: doctor (checks every external dependency and says what to fix), recalc (carbon for the last 35 days of every project, then recommendations),
// recommend (recommendation analysis for every project), refresh-grid (grid intensity), backfill <regions> [days] (past grid readings of any regions), partitions.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	carbondomain "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: admin doctor|azure-check|aws-check|gcp-check|recalc|recommend|refresh-grid|load-history|backfill|refresh-prices|refresh-fx|partitions")
		os.Exit(2)
	}
	if os.Args[1] == "azure-check" { // needs neither the database nor the platform configuration
		os.Exit(runAzureCheck(os.Args[2:]))
	}
	if os.Args[1] == "aws-check" || os.Args[1] == "gcp-check" {
		os.Exit(runCloudCheck(os.Args[1][:3], os.Args[2:]))
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
	case "refresh-fx":
		// The ECB euro reference rates (daily job; the first run loads the whole history), started now.
		run = func() error { return mux.ProcessTask(ctx, asynq.NewTask(queue.TaskRefreshFX, []byte("{}"))) }
	case "refresh-prices":
		// The relative EC2 price per region (weekly job), started now.
		run = func() error { return mux.ProcessTask(ctx, asynq.NewTask(queue.TaskRefreshPrices, []byte("{}"))) }
	case "load-history":
		// The daily automatic loading of the past of regions with gaps, started now (also redoes the calculations).
		run = func() error { return mux.ProcessTask(ctx, asynq.NewTask(queue.TaskAutoLoadHistory, []byte("{}"))) }
	case "backfill":
		// admin backfill <region>[,<region>...] [days]: loads the past grid readings of regions (also ones nobody uses yet).
		run = func() error {
			if len(os.Args) < 3 {
				return fmt.Errorf("usage: admin backfill <region>[,<region>...] [days]")
			}
			days := 30
			if len(os.Args) > 3 {
				if n, err := strconv.Atoi(os.Args[3]); err == nil && n > 0 {
					days = n
				}
			}
			to := time.Now().UTC().Truncate(time.Hour)
			n, err := c.BackfillIntensity(ctx, strings.Split(os.Args[2], ","), to.AddDate(0, 0, -days), to)
			log.Info("backfill", "stored_by_main_provider", n)
			if errors.Is(err, carbondomain.ErrPastUnavailable) {
				// the main provider's key or plan has no history; the free sources have loaded theirs (see grid_intensity)
				log.Warn("the main grid provider has no past data on this key or plan", "err", err)
				return nil
			}
			return err
		}
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
