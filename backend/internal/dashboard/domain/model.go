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
	// CostByProvider splits Cost over the clouds of the day (same currency as Cost): the chart stacks them, so two clouds are seen as two.
	CostByProvider map[string]float64 `json:"cost_by_provider"`
}

type ProviderShare struct {
	Provider string  `json:"provider"`
	CO2eKg   float64 `json:"carbon_kg_co2e"`
	SharePct float64 `json:"share_pct"`
}

type ServiceRow struct {
	Provider string    `json:"provider"` // the cloud (or vendor) the service is billed by: the same service name can exist in two clouds
	Service  string    `json:"service"`
	Category string    `json:"category"`
	Cost     float64   `json:"cost"`
	CO2eKg   float64   `json:"carbon_kg_co2e"`
	Spark    []float64 `json:"trend"` // daily CO2e, oldest first
}

// CountryIntensity is a country's latest published monthly national average (Ember), for colouring the map where no cloud region is.
type CountryIntensity struct {
	Code string `json:"code"` // ISO 3166 alpha-3
	// Source is "ember" (a published national monthly average) or "inventory" (national power emissions / generation of a year:
	// direct emissions, so a lower bound of the life-cycle figure).
	Source  string    `json:"source"`
	GPerKWh float64   `json:"g_per_kwh"`
	Month   time.Time `json:"month"`
}

type RegionIntensity struct {
	Region    string    `json:"region"`
	GPerKWh   float64   `json:"g_per_kwh"`
	ChangePct *float64  `json:"change_pct"` // vs 24h earlier; nil when unknown
	At        time.Time `json:"at"`
}

// RegionFootprint is what the tenant spends and emits in one cloud region over the period, with the grid intensity
// there and, when a clearly cleaner region of the same cloud exists, what moving the load there could save.
// The alternative is an estimate of the operational effect only: price, latency, residency and availability are
// not checked here, and the cost effect is never guessed.
type RegionFootprint struct {
	Provider    string       `json:"provider"`
	Region      string       `json:"region"`
	Cost        float64      `json:"cost"`
	EnergyKWh   float64      `json:"energy_kwh"`
	CO2eKg      float64      `json:"carbon_kg_co2e"`
	GPerKWh     *float64     `json:"g_per_kwh"` // latest grid reading; nil when none is stored
	Alternative *Alternative `json:"alternative"`
	// RelocationBlocked says why no cleaner region is proposed: "residency_unknown" (a project using the region has
	// no allow-list) or "residency" (no allowed region is clearly cleaner), or "latency" (cleaner regions exist but all are beyond the latency limit).
	RelocationBlocked string `json:"relocation_blocked,omitempty"`
}

type Alternative struct {
	Region        string  `json:"region"`
	GPerKWh       float64 `json:"g_per_kwh"`
	SavingKg      float64 `json:"carbon_saving_kg"`
	SavingPct     float64 `json:"carbon_saving_pct"`
	CostEstimated bool    `json:"cost_estimated"`
	// CostDelta is the estimated change of the EC2 compute cost in this period if that load ran in the candidate region
	// (list-price index; excludes discounts, data transfer and migration). Nil when no index or no EC2 cost.
	CostDelta *float64 `json:"cost_delta"`
	CostBasis string   `json:"cost_basis,omitempty"`
	// LatencyMs is the estimated round trip from where the load runs now to the candidate (from the distance; nil when a region
	// is not on the map), and LatencyBudgetMs the most the organization accepts. The move was chosen inside that budget.
	LatencyMs  *int `json:"latency_ms,omitempty"`
	DistanceKm *int `json:"distance_km,omitempty"`
	// LatencyBasis says where the figure comes from: measured_azure (Microsoft), measured_aws (CloudPing) or estimated (from distance,
	// typically off by LatencyErrorPct).
	LatencyBasis    string   `json:"latency_basis,omitempty"`
	LatencyErrorPct int      `json:"latency_error_pct,omitempty"`
	LatencyBudgetMs int      `json:"latency_budget_ms"`
	ResidencyOK     bool     `json:"residency_ok"` // every project using the source region allows the candidate
	ChecksRequired  []string `json:"checks_required"`
}

// Optimization is the value of the recommendations: potential (estimated, per month) and verified (measured after the
// change was applied, per 30 days) are kept apart and never added together.
type Optimization struct {
	OpenCount         int     `json:"open_count"`
	ApprovedCount     int     `json:"approved_count"`
	PotentialCarbonKg float64 `json:"potential_carbon_kg_month"`
	PotentialSaving   float64 `json:"potential_saving_month"`
	CostNotEstimated  int     `json:"cost_not_estimated_count"`
	AutoCount         int     `json:"auto_apply_count"` // open recommendations the projects' policies let the platform apply on its own
	VerifiedCount     int     `json:"verified_count"`
	NotRealizedCount  int     `json:"not_realized_count"`
	VerifiedCarbonKg  float64 `json:"verified_carbon_kg_month"`
	VerifiedSaving    float64 `json:"verified_saving_month"`
}

// Efficiency is cost and carbon per unit of work over the days the customer reported units for. Unit is empty (and
// the per-unit figures nil) when no units were reported or the projects count different things.
type Efficiency struct {
	HasUnits       bool     `json:"has_units"`
	Unit           string   `json:"unit"`
	Units          float64  `json:"units"`
	Days           int      `json:"days"`
	CarbonGPerUnit *float64 `json:"carbon_g_per_unit"`
	CostPerUnit    *float64 `json:"cost_per_unit"`
	MixedUnits     bool     `json:"mixed_units"`
}

type Activity struct {
	ID     int64     `json:"id"`
	Action string    `json:"action"`
	Target string    `json:"target"`
	At     time.Time `json:"at"`
	// Detail is what the event was about, from a short allowlist of the audit entry's own facts (a project's name, a report's
	// kind, the records a sync read). Nothing else of the audit metadata ever leaves.
	Detail map[string]string `json:"detail"`
}

// ActivityDetailKeys are the audit metadata keys Recent Activity may show.
var ActivityDetailKeys = []string{"name", "provider", "kind", "format", "type", "records", "amount", "period"}

// UserFacingActions is the allowlist of audit actions shown in Recent Activity.
// Security-sensitive or system actions (role changes, credential changes...) never appear here.
var UserFacingActions = []string{
	"project.created", "cloud_connection.created", "cloud_sync.completed",
	"report.generated", "recommendation.created", "recommendation.approved", "recommendation.applied",
	"budget.created", "project.deleted", "project.policy_updated",
}

type Filter struct {
	TenantID string
	Project  *string
	From, To time.Time
}

// ConnectionCost is what one cloud connection delivered over the period: its billed cost (organization currency),
// how many days have data and the newest day.
type ConnectionCost struct {
	ConnectionID string    `json:"connection_id"`
	Cost         float64   `json:"cost"`
	Days         int       `json:"days"`
	LastDay      time.Time `json:"last_day"`
	// Currency is what the connection is billed in and NativeCost the amount in it: an account billed in dollars is shown in dollars,
	// next to what it comes to in the reporting currency.
	Currency   string  `json:"currency"`
	NativeCost float64 `json:"native_cost"`
}

// GridSource is one provider of grid carbon intensity the platform has readings from.
type GridSource struct {
	Provider string    `json:"provider"`
	LastAt   time.Time `json:"last_at"`
	Regions  int       `json:"regions"`
}

type Repository interface {
	GridSources(ctx context.Context) ([]GridSource, error)
	ConnectionCosts(ctx context.Context, f Filter) ([]ConnectionCost, error)
	Trend(ctx context.Context, f Filter) ([]TrendPoint, error)
	Providers(ctx context.Context, f Filter) ([]ProviderShare, error)
	Services(ctx context.Context, f Filter, category string) ([]ServiceRow, error)
	Regions(ctx context.Context) ([]RegionIntensity, error)
	Countries(ctx context.Context) ([]CountryIntensity, error)
	RegionFootprint(ctx context.Context, f Filter) ([]RegionFootprint, error)
	Optimization(ctx context.Context, f Filter) (Optimization, error)
	Efficiency(ctx context.Context, f Filter) (Efficiency, error)
	UnitEconomics(ctx context.Context, tenantID string, project *string, months int) (UnitEconomics, error)
	Activity(ctx context.Context, tenantID string, limit int) ([]Activity, error)
}

var Categories = map[string]bool{"": true, "compute": true, "storage": true, "database": true, "networking": true, "other": true}
