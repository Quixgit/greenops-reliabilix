package domain

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Workload is the observed carbon and cost of one service in one region over a window.
type Workload struct {
	Provider, Service, Region string
	EnergyKWh, CarbonKg, Cost float64
}

// RegionShiftInput is everything the generator needs; it performs no I/O.
type RegionShiftInput struct {
	Workloads       []Workload
	WindowDays      int                // length of the observed window
	DataDays        int                // days of the window that actually have data
	Intensity       map[string]float64 // latest gCO2e/kWh per region
	AllowedRegions  []string           // project data-residency allow-list
	PriceIndex      map[string]float64 // optional relative price per region (provider-specific)
	MinReductionPct float64            // default 10
	MinMonthlyKg    float64            // default 1
	Now             time.Time
}

// GenerateRegionShift proposes moving workloads to a cleaner region.
//
// Hard rules:
//   - It never proposes a region outside the project's allow-list; with an empty list (residency unknown) it
//     proposes nothing at all.
//   - Cost impact is estimated only when a price index exists for both regions; otherwise it is reported
//     as not estimated rather than guessed.
//   - The carbon effect is computed from stored grid intensity, scaled to a month.
func GenerateRegionShift(in RegionShiftInput) []Recommendation {
	if len(in.AllowedRegions) == 0 || in.WindowDays <= 0 {
		return nil
	}
	minPct, minKg := in.MinReductionPct, in.MinMonthlyKg
	if minPct == 0 {
		minPct = 10
	}
	if minKg == 0 {
		minKg = 1
	}
	monthly := 30.0 / float64(in.WindowDays)
	dataFactor := math.Min(1, float64(in.DataDays)/30)

	var out []Recommendation
	for _, w := range in.Workloads {
		cur, ok := in.Intensity[w.Region]
		if !ok || cur <= 0 || w.CarbonKg <= 0 {
			continue
		}
		target, tIntensity := "", cur
		for _, r := range in.AllowedRegions { // pick the cleanest allowed region
			if r == w.Region {
				continue
			}
			if g, ok := in.Intensity[r]; ok && g < tIntensity {
				target, tIntensity = r, g
			}
		}
		if target == "" {
			continue
		}
		pct := (cur - tIntensity) / cur * 100
		kg := w.CarbonKg * monthly * pct / 100
		if pct < minPct || kg < minKg {
			continue
		}
		rec := Recommendation{
			Type: RegionShift, Title: fmt.Sprintf("Move %s from %s to %s", w.Service, w.Region, target),
			Provider: w.Provider, ServiceName: w.Service, CurrentRegion: w.Region, RecommendedRegion: target,
			CarbonReductionPct: round(pct, 1), CarbonReductionKg: round(kg, 1), CostBasis: "not_estimated", Status: Open,
			Confidence: round(math.Min(0.9, (0.5+0.4*math.Min(1, pct/50))*dataFactor), 2),
			Compliance: ComplianceCheck{Residency: "passed", AllowedRegions: append([]string(nil), in.AllowedRegions...), CheckedAt: in.Now},
		}
		if a, b := in.PriceIndex[w.Region], in.PriceIndex[target]; a > 0 && b > 0 {
			impact := round(w.Cost*monthly*(b/a-1), 2)
			rec.CostImpact, rec.CostBasis = &impact, "region_price_index"
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CarbonReductionKg > out[j].CarbonReductionKg })
	return out
}

func round(v float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(v*p) / p
}
