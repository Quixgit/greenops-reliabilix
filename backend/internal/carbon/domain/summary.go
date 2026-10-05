package domain

import "time"

// Totals aggregates calculations over a window.
type Totals struct {
	EnergyKWh        float64
	CO2eKg           float64
	IntensityGPerKWh float64  // effective: CO2e / energy
	SCI              *float64 // gCO2e per functional unit; nil until the customer reports functional units
	Rows             int64
	Days             int // distinct days that have data
}

// Summary is the window and the immediately preceding window of equal length.
type Summary struct {
	Current, Previous Totals
}

func (s Summary) HasData() bool { return s.Current.Rows > 0 }

// MinCoverage is the share of days a window must have data for before it is compared with another window.
const MinCoverage = 0.8

// Comparable reports whether the current and previous windows are both (nearly) fully covered by data.
// Comparing a full month with a partially synced one would show a fake change (e.g. +500%), so the
// API says "not comparable" and clients must not draw a delta.
func Comparable(currentDays, previousDays, windowDays int) bool {
	need := int(float64(windowDays)*MinCoverage + 0.5)
	return windowDays > 0 && currentDays >= need && previousDays >= need
}

type TrendPoint struct {
	Day    time.Time `json:"day"`
	CO2eKg float64   `json:"carbon_kg_co2e"`
}

// Thresholds are the CI-gate limits of a project (nil = not configured). Values are per month.
type Thresholds struct {
	CarbonWarnKg, CarbonBlockKg *float64
	CostWarn, CostBlock         *float64
}

func (t Thresholds) Any() bool {
	return t.CarbonWarnKg != nil || t.CarbonBlockKg != nil || t.CostWarn != nil || t.CostBlock != nil
}

// Verdict is the CI gate outcome.
type Verdict string

const (
	Pass  Verdict = "PASS"
	Warn  Verdict = "WARN"
	Block Verdict = "BLOCK"
)

// Gate evaluates a change's monthly carbon and cost deltas against thresholds. Only increases count:
// a change that reduces carbon or cost is never blocked. BLOCK outranks WARN.
func Gate(carbonKg, costDelta float64, t Thresholds) (Verdict, []string) {
	v, reasons := Pass, []string{}
	check := func(val float64, limit *float64, level Verdict, label string) {
		if limit == nil || val <= *limit {
			return
		}
		reasons = append(reasons, label)
		if level == Block || v == Pass {
			v = level
		}
	}
	check(carbonKg, t.CarbonWarnKg, Warn, "carbon increase exceeds the warning threshold")
	check(costDelta, t.CostWarn, Warn, "cost increase exceeds the warning threshold")
	check(carbonKg, t.CarbonBlockKg, Block, "carbon increase exceeds the blocking threshold")
	check(costDelta, t.CostBlock, Block, "cost increase exceeds the blocking threshold")
	return v, reasons
}
