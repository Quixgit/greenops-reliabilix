package domain

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// Quality says how far one sync's cost data can be trusted (ADR-0024). It compares the billed cost that was stored
// with the day totals the cloud itself reports.
type Quality struct {
	// Status: "ok" (every checked day matches), "mismatch" (some days differ) or "unchecked" (the provider has no
	// independent total to compare with, or it could not be fetched).
	Status        string             `json:"status"`
	CheckedDays   int                `json:"checked_days"`
	MismatchDays  int                `json:"mismatch_days"`
	MaxDiffPct    float64            `json:"max_diff_pct"` // largest difference as a percentage of that day's cloud total
	MismatchedOn  []string           `json:"mismatched_days,omitempty"`
	ProviderTotal map[string]float64 `json:"provider_total,omitempty"` // per currency, checked days only
	StoredTotal   map[string]float64 `json:"stored_total,omitempty"`
	Note          string             `json:"note,omitempty"`
	CheckedAt     time.Time          `json:"checked_at"`
}

const (
	QualityOK        = "ok"
	QualityMismatch  = "mismatch"
	QualityUnchecked = "unchecked"
)

// Tolerances: costs are summed from many lines and rounded by the provider, so tiny differences are normal.
const (
	tolerancePct = 0.5  // percent of the day's total
	toleranceAbs = 0.05 // in the billing currency
	maxListed    = 10
)

// ControlProvider is implemented by providers that can report the cloud's own day totals independently of the
// detailed query (for example a Cost Explorer query without grouping).
type ControlProvider interface {
	GetControlTotals(ctx context.Context, req UsageRequest) ([]focus.ControlTotal, error)
}

// Unchecked builds the quality of a sync that could not be verified, with the reason.
func Unchecked(note string, at time.Time) Quality {
	return Quality{Status: QualityUnchecked, Note: note, CheckedAt: at}
}

// Reconcile compares the cloud's totals (control) with what was stored. A day that has a control total but no stored
// cost counts as a full mismatch: that is exactly the "missing page" failure this check exists for.
func Reconcile(control, stored []focus.ControlTotal, at time.Time) Quality {
	type key struct{ day, cur string }
	have := map[key]float64{}
	for _, s := range stored {
		have[key{s.Day, s.Currency}] += s.Billed
	}
	q := Quality{Status: QualityOK, ProviderTotal: map[string]float64{}, StoredTotal: map[string]float64{}, CheckedAt: at}
	if len(control) == 0 {
		return Unchecked("the provider reported no day totals for this window", at)
	}
	var bad []string
	for _, c := range control {
		got := have[key{c.Day, c.Currency}]
		q.CheckedDays++
		q.ProviderTotal[c.Currency] += c.Billed
		q.StoredTotal[c.Currency] += got
		diff := math.Abs(got - c.Billed)
		if diff <= math.Max(toleranceAbs, math.Abs(c.Billed)*tolerancePct/100) {
			continue
		}
		q.MismatchDays++
		bad = append(bad, c.Day)
		if pct := diff / math.Max(math.Abs(c.Billed), toleranceAbs) * 100; pct > q.MaxDiffPct {
			q.MaxDiffPct = pct
		}
	}
	if q.MismatchDays > 0 {
		q.Status = QualityMismatch
		sort.Strings(bad)
		if len(bad) > maxListed {
			bad = bad[:maxListed]
		}
		q.MismatchedOn = bad
		q.Note = fmt.Sprintf("%d of %d days differ from the cloud's own totals; the next sync re-reads them", q.MismatchDays, q.CheckedDays)
	}
	for cur, v := range q.ProviderTotal {
		q.ProviderTotal[cur] = round2(v)
	}
	for cur, v := range q.StoredTotal {
		q.StoredTotal[cur] = round2(v)
	}
	q.MaxDiffPct = round2(q.MaxDiffPct)
	return q
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// EarliestMismatch returns the first mismatched day, so the sync cursor can be rewound to re-read from there.
func (q Quality) EarliestMismatch() (time.Time, bool) {
	var first time.Time
	for _, d := range q.MismatchedOn { // listed in ascending order
		t, err := time.Parse(time.DateOnly, d)
		if err != nil {
			continue
		}
		if first.IsZero() || t.Before(first) {
			first = t
		}
	}
	return first, !first.IsZero()
}
