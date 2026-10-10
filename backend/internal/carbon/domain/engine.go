package domain

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
)

// UsageDay is one day of one workload dimension (provider, service, region), as aggregated by the usage domain.
type UsageDay struct {
	Provider, ServiceName, ServiceCategory, RegionID, Currency string
	ConsumedUnit                                               *string
	Day                                                        time.Time
	Quantity                                                   float64
	HasQuantity                                                bool
	BilledCost                                                 float64
}

// Calculation is the engine's output row; it mirrors carbon.calculations.
type Calculation struct {
	Provider, ServiceName, ServiceCategory, RegionID string
	Method                                           Method
	PeriodStart, PeriodEnd                           time.Time
	EnergyKWh, IntensityGPerKWh, EmbodiedG, CO2eKg   float64
	FunctionalUnits                                  *float64 // SCI "R" of the day, when reported
	SCIScore                                         *float64 // this row's share of the day's SCI (g per unit)
	MethodologyVersion                               string
	IntensitySource, IntensityBasis                  string // where IntensityGPerKWh came from (see Basis*)
}

// Skip reasons: rows the engine refuses to estimate instead of guessing.
const (
	SkipNoRegion   = "no_region"            // global services have no grid to attribute to
	SkipNoGridData = "no_grid_data"         // no carbon intensity known for the region
	SkipCurrency   = "unsupported_currency" // cost in a currency without a reference rate (cost is converted to USD before the engine sees it)
	SkipNoMeasure  = "no_measurable_usage"
	// SkipSuperseded: a cost-based row was dropped because measured usage exists for the same
	// service, region and day. Counting both would double the energy.
	SkipSuperseded = "superseded_by_measured_usage"
)

// IntensityLookup returns the grid intensity (gCO2e/kWh) a day of usage ran under, with its source and basis. at is
// the END of the usage day. ok=false when unknown.
type IntensityLookup func(ctx context.Context, region string, at time.Time) (ref IntensityRef, ok bool, err error)

// Compute turns daily usage into calculations. unitsByDay holds SCI's R per day (missing/0 = unknown, so
// SCI stays NULL). It is pure apart from the IntensityLookup callback, which makes it fully testable.
func Compute(ctx context.Context, rows []UsageDay, lookup IntensityLookup, unitsByDay map[time.Time]float64) (out []Calculation, skipped map[string]int, err error) {
	return ComputeWith(Current, ctx, rows, lookup, unitsByDay)
}

// ComputeWith is Compute under an explicit coefficient set; every row carries that set's version.
func ComputeWith(f methodology.Factors, ctx context.Context, rows []UsageDay, lookup IntensityLookup, unitsByDay map[time.Time]float64) (out []Calculation, skipped map[string]int, err error) {
	skipped = map[string]int{}
	measured := measuredKeys(rows)
	for _, u := range rows {
		if u.RegionID == "" || u.RegionID == "global" {
			skipped[SkipNoRegion]++
			continue
		}
		var (
			kwh    float64
			method Method
		)
		if kind, ok := measurableKind(u); ok {
			kwh, err = EnergyKWhWith(f, kind, u.Quantity)
			if err != nil {
				return nil, nil, err
			}
			method = UsageBased
		} else {
			if measured[workloadKey(u)] {
				skipped[SkipSuperseded]++
				continue
			}
			if u.Currency != "USD" {
				skipped[SkipCurrency]++
				continue
			}
			var ok bool
			if kwh, ok = CostBasedEnergyKWhWith(f, u.ServiceCategory, u.BilledCost); !ok {
				skipped[SkipNoMeasure]++
				continue
			}
			method = CostBased
		}
		end := u.Day.Add(24 * time.Hour)
		ref, ok, err := lookup(ctx, u.RegionID, end)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			skipped[SkipNoGridData]++
			continue
		}
		g := ref.G
		embodied := kwh * f.EmbodiedGPerKWh
		grams := kwh*g + embodied
		c := Calculation{
			Provider: u.Provider, ServiceName: u.ServiceName, ServiceCategory: u.ServiceCategory, RegionID: u.RegionID, Method: method,
			PeriodStart: u.Day, PeriodEnd: end, EnergyKWh: kwh, IntensityGPerKWh: g, EmbodiedG: embodied, CO2eKg: grams / 1000,
			MethodologyVersion: f.Version, IntensitySource: ref.Source, IntensityBasis: ref.Basis,
		}
		if r := unitsByDay[u.Day]; r > 0 {
			sci := grams / r // SCI = ((E*I)+M)/R, this row's share of the day
			c.FunctionalUnits, c.SCIScore = &r, &sci
		}
		out = append(out, c)
	}
	return out, skipped, nil
}

// workloadKey identifies the service, region and day that a measured row and a cost row share.
func workloadKey(u UsageDay) string {
	return u.Provider + "|" + u.ServiceName + "|" + u.RegionID + "|" + u.Day.Format(time.DateOnly)
}

// measuredKeys lists the workloads that have at least one measured (usage-based) row.
func measuredKeys(rows []UsageDay) map[string]bool {
	out := map[string]bool{}
	for _, u := range rows {
		if _, ok := measurableKind(u); ok {
			out[workloadKey(u)] = true
		}
	}
	return out
}

func measurableKind(u UsageDay) (UsageKind, bool) {
	if !u.HasQuantity || u.ConsumedUnit == nil {
		return "", false
	}
	return KindForUnit(*u.ConsumedUnit)
}

// SCI computes the Software Carbon Intensity per functional unit: SCI = ((E * I) + M) / R, in gCO2e per R.
func SCI(energyKWh, intensityGPerKWh, embodiedG, units float64) (float64, error) {
	if units <= 0 {
		return 0, ErrInvalidInput
	}
	if energyKWh < 0 || intensityGPerKWh < 0 || embodiedG < 0 {
		return 0, ErrInvalidInput
	}
	return (energyKWh*intensityGPerKWh + embodiedG) / units, nil
}
