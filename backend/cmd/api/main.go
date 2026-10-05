// Command api is the HTTP API process.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/automation"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts"
	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard"
	"github.com/quixgit/greenops-reliabilix/backend/internal/finops"
	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion"
	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/users"
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
	flush, err := observability.InitTracing(ctx, "api", cfg.OTLPEndpoint)
	if err != nil {
		log.Error("tracing", "err", err)
		os.Exit(1)
	}
	defer func() { _ = flush(context.Background()) }()

	var verifier auth.Verifier = auth.NewOIDCVerifier(cfg.Auth0Domain, cfg.Auth0Audience, cfg.ClaimNS)
	if cfg.IsDev() {
		verifier = auth.DevVerifier{}
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
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
	default:
		log.Warn("no ELECTRICITYMAPS_API_KEY: /carbon/calculate will fail closed")
	}

	modules := []httpx.Module{
		tenants.New(pool), users.New(), projects.New(pool), cloudaccounts.New(pool, log),
		ingestion.New(), usage.New(pool), carbon.New(pool, grid, log), finops.New(pool),
		dashboard.New(pool), recommendations.New(), kubernetes.New(), reports.New(), automation.New(),
	}
	router := app.NewRouter(cfg, log, verifier, pool.Ping, modules...)

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
