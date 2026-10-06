// Package focus is the shared cost-and-usage contract. Field names follow FOCUS v1.4
// (FinOps Open Cost and Usage Specification): every cloud connector is normalized into
// Record, so no domain depends on a provider-specific billing format.
package focus

import (
	"errors"
	"fmt"
	"time"
)

// Record is one charge line (FOCUS: a row of the cost and usage dataset).
type Record struct {
	Provider          string // aws | azure | gcp | k8s
	Source            string // cost_explorer | cur | ...
	ResourceID        string // FOCUS ResourceId ("" when the source has no resource granularity)
	ResourceType      string // FOCUS ResourceType
	ServiceName       string // FOCUS ServiceName
	ServiceCategory   string // FOCUS ServiceCategory (reduced set: compute|storage|database|networking|other)
	RegionID          string // FOCUS RegionId
	ConsumedQuantity  *float64
	ConsumedUnit      *string
	BilledCost        float64 // FOCUS BilledCost; may be negative (credits)
	EffectiveCost     float64 // FOCUS EffectiveCost
	Currency          string  // FOCUS BillingCurrency, ISO 4217
	ChargePeriodStart time.Time
	ChargePeriodEnd   time.Time
}

// RawBatch is an unprocessed provider payload, archived and then normalized.
type RawBatch struct {
	Provider string
	Source   string
	Payload  []byte
}

var categories = map[string]bool{"compute": true, "storage": true, "database": true, "networking": true, "other": true}

var providers = map[string]bool{"aws": true, "azure": true, "gcp": true, "k8s": true}

// Validate enforces the invariants required before a record may be persisted.
func (r Record) Validate() error {
	switch {
	case !providers[r.Provider]:
		return fmt.Errorf("focus: unknown provider %q", r.Provider)
	case r.Source == "":
		return errors.New("focus: source required")
	case r.ServiceName == "":
		return errors.New("focus: service_name required")
	case !categories[r.ServiceCategory]:
		return fmt.Errorf("focus: unknown service_category %q", r.ServiceCategory)
	case r.RegionID == "":
		return errors.New("focus: region_id required")
	case len(r.Currency) != 3:
		return errors.New("focus: currency must be ISO 4217")
	case !r.ChargePeriodEnd.After(r.ChargePeriodStart):
		return errors.New("focus: charge_period_end must be after charge_period_start")
	case r.ConsumedQuantity != nil && *r.ConsumedQuantity < 0:
		return errors.New("focus: consumed_quantity must be non-negative")
	case (r.ConsumedQuantity == nil) != (r.ConsumedUnit == nil):
		return errors.New("focus: consumed_quantity and consumed_unit go together")
	}
	return nil
}

// IngestResult describes what one ingestion run wrote.
type IngestResult struct {
	Records  int
	From, To time.Time // first and last charge day touched
}
