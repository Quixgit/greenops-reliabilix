// Package focus is the shared cost-and-usage contract. Field names follow FOCUS v1.4
// (FinOps Open Cost and Usage Specification): every cloud connector is normalized into
// Record, so no domain depends on a provider-specific billing format.
package focus

import (
	"errors"
	"fmt"
	"math"
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
	case !validCurrency(r.Currency):
		return errors.New("focus: currency must be ISO 4217 (three upper-case letters)")
	case !finite(r.BilledCost) || !finite(r.EffectiveCost):
		// ParseFloat accepts "NaN" and "Inf": a provider payload must never smuggle them into the database.
		return errors.New("focus: costs must be finite numbers")
	case r.ConsumedQuantity != nil && !finite(*r.ConsumedQuantity):
		return errors.New("focus: consumed_quantity must be a finite number")
	case !r.ChargePeriodEnd.After(r.ChargePeriodStart):
		return errors.New("focus: charge_period_end must be after charge_period_start")
	case r.ConsumedQuantity != nil && *r.ConsumedQuantity < 0:
		return errors.New("focus: consumed_quantity must be non-negative")
	case (r.ConsumedQuantity == nil) != (r.ConsumedUnit == nil):
		return errors.New("focus: consumed_quantity and consumed_unit go together")
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func validCurrency(c string) bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ControlTotal is the billed cost of one day in one currency. Providers report these straight from the cloud's own
// totals (a second, ungrouped query), and ingestion reports what it actually stored; comparing the two catches
// truncated pages, dropped rows and parsing mistakes that would otherwise silently understate costs.
type ControlTotal struct {
	Day      string // YYYY-MM-DD (UTC)
	Currency string
	Billed   float64
}

// IngestResult describes what one ingestion run wrote.
type IngestResult struct {
	Records  int
	From, To time.Time // first and last charge day touched
	// Totals is the billed cost that was stored, per day and currency, for reconciliation with the cloud's totals.
	Totals []ControlTotal
}
