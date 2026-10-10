// Package observability wires logs, traces (OpenTelemetry) and Prometheus metrics.
package observability

import (
	"context"
	"fmt"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/version"
	"time"

	"github.com/getsentry/sentry-go"
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Logger returns a JSON logger tagged with the process name.
func Logger(process string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("process", process)
}

// InitTracing configures the global tracer provider. Without an OTLP endpoint
// tracing stays a no-op. The returned func flushes spans on shutdown.
func InitTracing(ctx context.Context, process, endpoint string) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceName("greenops-"+process))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// MetricsHandler serves Prometheus metrics (bind it to an internal-only address).
func MetricsHandler() http.Handler { return promhttp.Handler() }

// Business metrics (the data behind the service-extraction triggers, ADR-0005).
var (
	CloudSyncTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloud_sync_total", Help: "Cloud sync runs by provider and result."}, []string{"provider", "result"})
	CloudSyncReconciliation = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloud_sync_reconciliation_total", Help: "Cost reconciliation of cloud syncs by provider and result (ok, mismatch, unchecked)."}, []string{"provider", "result"})
	CloudSyncDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "cloud_sync_duration_seconds", Help: "Cloud sync duration.", Buckets: prometheus.ExponentialBuckets(1, 2, 10)}, []string{"provider"})
	UsageRecordsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "usage_records_processed_total", Help: "Normalized usage records persisted."})
	CarbonCalculations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "carbon_calculations_total", Help: "Carbon calculations performed."})
	KubernetesSamples = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kubernetes_energy_samples_total", Help: "Workload energy samples accepted from agents.",
	})
	RecommendationsGenerated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "recommendations_generated_total", Help: "Recommendations generated."})
	CarbonAPIErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "carbon_api_errors_total", Help: "Errors from the grid carbon data provider."})
	ErrorsRecorded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "platform_errors_total", Help: "Errors written to the operator error log by service and result (recorded, dropped, failed)."}, []string{"service", "result"})
	NotificationDeliveries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notification_deliveries_total", Help: "Notification delivery attempts by channel and outcome (sent, retry, failed, skipped)."}, []string{"channel", "outcome"})
)

// RegisterBuildInfo publishes build_info{service,version,stage,commit} = 1 so dashboards and alerts can see which
// version every process is running (docs/release/versioning.md).
func RegisterBuildInfo() {
	i := version.Current()
	g := promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "build_info", Help: "Build identity of the running process (value is always 1)."}, []string{"service", "version", "stage", "commit"})
	g.WithLabelValues(i.Service, i.Platform, i.Stage, i.Commit).Set(1)
}

// InitSentry enables error tracking when a DSN is configured. The returned func flushes pending events.
// PII is not sent (the SDK default); request bodies and headers are never attached.
func InitSentry(dsn, env, release string) (func(), error) {
	if dsn == "" {
		return func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{Dsn: dsn, Environment: env, Release: release, AttachStacktrace: true}); err != nil {
		return nil, fmt.Errorf("sentry: %w", err)
	}
	return func() { sentry.Flush(2 * time.Second) }, nil
}

// CaptureError reports an unexpected error (no-op when Sentry is not initialised).
func CaptureError(err error) { sentry.CaptureException(err) }
