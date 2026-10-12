// Package focus is the shared cost-and-usage contract. Field names follow FOCUS v1.4
// (FinOps Open Cost and Usage Specification): every cloud connector is normalized into
// Record, so no domain depends on a provider-specific billing format.
package focus

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Record is one charge line (FOCUS: a row of the cost and usage dataset).
type Record struct {
	Provider          string // aws | azure | gcp | k8s | saas
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
	// TagFrom is the first day (UTC date) the payload's tags cover when that is earlier than its cost lines: a source that reads tags in a
	// separate, wider query says so, and what it reports for those days replaces what was stored. Zero: the cost lines' own days.
	TagFrom time.Time
	// Meta carries what a payload cannot say about itself (a CSV file does not name its vendor): small strings set by whoever made the batch.
	Meta map[string]string
}

var categories = map[string]bool{"compute": true, "storage": true, "database": true, "networking": true, "other": true}

// "saas" is every vendor outside the three clouds (Datadog, Snowflake ...): cost only, no region, no carbon model.
var providers = map[string]bool{"aws": true, "azure": true, "gcp": true, "k8s": true, "saas": true}

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

// TaggedCost is the cost of one day and service that carried one value of one cloud tag (or label): the input of showback. It is
// kept apart from Record so that a charge is never counted twice: Records are the invoice, TaggedCosts are a view of the same money by tag.
type TaggedCost struct {
	Provider      string
	ServiceName   string
	Currency      string
	Day           time.Time // the charge period start, UTC midnight
	TagKey        string    // normalized (see NormalizeTagKey)
	TagValue      string    // trimmed, at most MaxTagValue characters
	BilledCost    float64
	EffectiveCost float64
}

// Limits of a tag as stored: a tag is customer input.
const (
	MaxTagKey   = 64
	MaxTagValue = 128
)

// NormalizeTagKey makes the spellings of one tag equal: lower case, and the prefix some clouds add to user-defined tags removed
// ("user_team", "user:team" and "Team" are all "team"). Tags the cloud itself sets ("aws:...", "goog-...", "microsoft.") are not the customer's
// allocation and give "". A key longer than MaxTagKey gives "".
func NormalizeTagKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	for _, sys := range []string{"aws:", "goog-", "microsoft.", "azure:", "kubernetes.io/", "k8s.io/"} {
		if strings.HasPrefix(k, sys) {
			return ""
		}
	}
	k = strings.TrimPrefix(strings.TrimPrefix(k, "user_"), "user:")
	if k == "" || len(k) > MaxTagKey {
		return ""
	}
	return k
}

// SelectTags keeps the tags whose normalized key is in keys (already normalized), as key to trimmed value. An empty value is not a value:
// the charge is then untagged for that key.
func SelectTags(tags map[string]string, keys map[string]bool) map[string]string {
	var out map[string]string
	for k, v := range tags {
		nk := NormalizeTagKey(k)
		v = strings.TrimSpace(v)
		if nk == "" || !keys[nk] || v == "" {
			continue
		}
		if len(v) > MaxTagValue {
			v = v[:MaxTagValue]
		}
		if out == nil {
			out = map[string]string{}
		}
		out[nk] = v
	}
	return out
}

// KeySet turns a list of tag keys into the set SelectTags wants, normalizing each.
func KeySet(keys []string) map[string]bool {
	out := map[string]bool{}
	for _, k := range keys {
		if nk := NormalizeTagKey(k); nk != "" {
			out[nk] = true
		}
	}
	return out
}

// Validate enforces what may be stored.
func (t TaggedCost) Validate() error {
	switch {
	case !providers[t.Provider]:
		return fmt.Errorf("focus: unknown provider %q", t.Provider)
	case t.ServiceName == "":
		return errors.New("focus: tagged cost needs a service name")
	case !validCurrency(t.Currency):
		return errors.New("focus: currency must be ISO 4217 (three upper-case letters)")
	case t.TagKey == "" || len(t.TagKey) > MaxTagKey || t.TagValue == "" || len(t.TagValue) > MaxTagValue:
		return errors.New("focus: tag key and value are required and bounded")
	case !finite(t.BilledCost) || !finite(t.EffectiveCost):
		return errors.New("focus: costs must be finite numbers")
	}
	return nil
}

// TagNormalizer is implemented by a normalizer whose payload also carries cost by tag.
type TagNormalizer interface {
	NormalizeTags(raw RawBatch) ([]TaggedCost, error)
}
