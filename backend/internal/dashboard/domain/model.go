// Package domain defines the read models of the Overview screen. The dashboard
// module is a deliberate read-only composition over the usage, carbon and audit
// schemas: it owns no tables and never writes.
package domain

import (
	"context"
	"time"
)

type TrendPoint struct {
	Day    time.Time `json:"day"`
	Cost   float64   `json:"cost"`
	CO2eKg float64   `json:"carbon_kg_co2e"`
}

type ProviderShare struct {
	Provider string  `json:"provider"`
	CO2eKg   float64 `json:"carbon_kg_co2e"`
	SharePct float64 `json:"share_pct"`
}

type ServiceRow struct {
	Service  string    `json:"service"`
	Category string    `json:"category"`
	Cost     float64   `json:"cost"`
	CO2eKg   float64   `json:"carbon_kg_co2e"`
	Spark    []float64 `json:"trend"` // daily CO2e, oldest first
}

type RegionIntensity struct {
	Region    string    `json:"region"`
	GPerKWh   float64   `json:"g_per_kwh"`
	ChangePct *float64  `json:"change_pct"` // vs 24h earlier; nil when unknown
	At        time.Time `json:"at"`
}

type Activity struct {
	ID     int64     `json:"id"`
	Action string    `json:"action"`
	Target string    `json:"target"`
	At     time.Time `json:"at"`
}

// UserFacingActions is the allowlist of audit actions shown in Recent Activity.
// Security-sensitive or system actions (role changes, credential changes...) never appear here.
var UserFacingActions = []string{
	"project.created", "cloud_connection.created", "cloud_sync.completed",
	"report.generated", "recommendation.created", "recommendation.applied",
}

type Filter struct {
	TenantID string
	Project  *string
	From, To time.Time
}

type Repository interface {
	Trend(ctx context.Context, f Filter) ([]TrendPoint, error)
	Providers(ctx context.Context, f Filter) ([]ProviderShare, error)
	Services(ctx context.Context, f Filter, category string) ([]ServiceRow, error)
	Regions(ctx context.Context) ([]RegionIntensity, error)
	Activity(ctx context.Context, tenantID string, limit int) ([]Activity, error)
}

var Categories = map[string]bool{"": true, "compute": true, "storage": true, "database": true, "networking": true, "other": true}
