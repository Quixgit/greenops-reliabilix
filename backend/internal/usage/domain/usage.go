// Package domain defines stored usage in FOCUS terms and how it is queried.
package domain

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// Record is a stored FOCUS charge line (JSON names follow FOCUS column names).
type Record struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	Provider          string    `json:"provider"`
	Source            string    `json:"source"`
	ResourceID        string    `json:"resource_id"`
	ResourceType      string    `json:"resource_type"`
	ServiceName       string    `json:"service_name"`
	ServiceCategory   string    `json:"service_category"`
	RegionID          string    `json:"region_id"`
	ConsumedQuantity  *float64  `json:"consumed_quantity"`
	ConsumedUnit      *string   `json:"consumed_unit"`
	BilledCost        float64   `json:"billed_cost"`
	EffectiveCost     float64   `json:"effective_cost"`
	Currency          string    `json:"currency"`
	ChargePeriodStart time.Time `json:"charge_period_start"`
	ChargePeriodEnd   time.Time `json:"charge_period_end"`
}

// DailyAggregate is a per-day rollup of one workload dimension: the carbon engine's input.
type DailyAggregate struct {
	Provider, ServiceName, ServiceCategory, RegionID, Currency string
	ConsumedUnit                                               *string
	Day                                                        time.Time
	Quantity                                                   float64
	HasQuantity                                                bool
	BilledCost                                                 float64
}

type Repository interface {
	// Upsert writes records idempotently (re-syncing the same day replaces, never duplicates).
	Upsert(ctx context.Context, tenantID, projectID, connectionID string, recs []focus.Record) error
	List(ctx context.Context, tenantID string, projectID *string, from, to time.Time, limit int) ([]Record, error)
	DailyAggregates(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]DailyAggregate, error)
}
