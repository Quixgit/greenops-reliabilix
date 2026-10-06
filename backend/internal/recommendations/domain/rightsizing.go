package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"time"
)

// hoursPerMonth converts a running instance into monthly energy (24 x 365 / 12).
const hoursPerMonth = 730.0

// Shape is the hardware of an instance type (zero = unknown).
type Shape struct{ VCPU, MemoryGB float64 }

func (s Shape) known() bool { return s.VCPU > 0 && s.MemoryGB > 0 }

// EnergyModel converts a shape into energy. Coefficients come from the active carbon methodology.
type EnergyModel struct {
	KWhPerVCPUHour float64
	KWhPerGBHour   float64
	PUE            float64
	// ShapeOf resolves a type the provider did not describe (optional).
	ShapeOf func(instanceType string) (Shape, bool)
}

// monthlyKWh is the modelled energy of one always-on instance for a month.
func (m EnergyModel) monthlyKWh(s Shape) float64 {
	return hoursPerMonth * (s.VCPU*m.KWhPerVCPUHour + s.MemoryGB*m.KWhPerGBHour) * m.PUE
}

func (m EnergyModel) resolve(reported Shape, instanceType string) (Shape, bool) {
	if reported.known() {
		return reported, true
	}
	if m.ShapeOf != nil && instanceType != "" {
		return m.ShapeOf(instanceType)
	}
	return Shape{}, false
}

// RightsizingFinding is one provider recommendation for a single instance.
type RightsizingFinding struct {
	ResourceID              string
	Region                  string
	CurrentType, TargetType string // TargetType empty = terminate
	Current, Target         Shape
	EstimatedMonthlySavings float64
	Currency                string
}

// RightsizingInput is everything the generator needs; it performs no I/O.
type RightsizingInput struct {
	ProjectID    string
	Findings     []RightsizingFinding
	Intensity    map[string]float64 // latest gCO2e/kWh per region
	Model        EnergyModel
	MinMonthlyKg float64 // default 0.5: below this the advice is noise
	Provider     string
	Now          time.Time
}

// RightsizingResult separates what was produced from what was left out and why.
type RightsizingResult struct {
	Recommendations []Recommendation
	Skipped         map[string]int
}

// Reasons a finding is not turned into a recommendation.
const (
	SkipNoShape     = "unknown_instance_shape"
	SkipNoIntensity = "no_grid_data"
	SkipTooSmall    = "below_threshold"
	SkipNoReduction = "no_energy_reduction"
)

// GenerateRightsizing turns provider findings into recommendations with a modelled carbon effect.
//
// Hard rules:
//   - The carbon effect is modelled from the instance shape and the stored grid intensity of its region; a
//     finding whose shape or region intensity is unknown is skipped, never guessed.
//   - A modification that does not reduce modelled energy (different family, same size) is skipped.
//   - The cost effect is the provider's own estimate, and only when it is in USD; otherwise it is reported
//     as not estimated.
//   - The region does not change, so data residency is trivially satisfied.
func GenerateRightsizing(in RightsizingInput) RightsizingResult {
	res := RightsizingResult{Skipped: map[string]int{}}
	minKg := in.MinMonthlyKg
	if minKg == 0 {
		minKg = 0.5
	}
	for _, f := range in.Findings {
		cur, ok := in.Model.resolve(f.Current, f.CurrentType)
		if !ok {
			res.Skipped[SkipNoShape]++
			continue
		}
		var target Shape // zero for terminate: nothing keeps running
		if f.TargetType != "" {
			if target, ok = in.Model.resolve(f.Target, f.TargetType); !ok {
				res.Skipped[SkipNoShape]++
				continue
			}
		}
		g, ok := in.Intensity[f.Region]
		if !ok || g <= 0 {
			res.Skipped[SkipNoIntensity]++
			continue
		}
		before := in.Model.monthlyKWh(cur)
		after := 0.0
		if f.TargetType != "" {
			after = in.Model.monthlyKWh(target)
		}
		savedKWh := before - after
		if savedKWh <= 0 || before <= 0 {
			res.Skipped[SkipNoReduction]++
			continue
		}
		kg := savedKWh * g / 1000
		if kg < minKg {
			res.Skipped[SkipTooSmall]++
			continue
		}
		res.Recommendations = append(res.Recommendations, buildRightsizing(in, f, round(savedKWh/before*100, 1), round(kg, 1)))
	}
	sort.Slice(res.Recommendations, func(i, j int) bool {
		return res.Recommendations[i].CarbonReductionKg > res.Recommendations[j].CarbonReductionKg
	})
	return res
}

func buildRightsizing(in RightsizingInput, f RightsizingFinding, pct, kg float64) Recommendation {
	title := fmt.Sprintf("Terminate idle instance %s (%s)", f.ResourceID, f.CurrentType)
	confidence := 0.7
	if f.TargetType != "" {
		title = fmt.Sprintf("Resize %s from %s to %s", f.ResourceID, f.CurrentType, f.TargetType)
		confidence = 0.8
	}
	r := Recommendation{
		Type: Rightsizing, Title: title, Provider: in.Provider, ServiceName: "Amazon EC2",
		CurrentRegion: f.Region, RecommendedRegion: f.Region,
		CarbonReductionPct: pct, CarbonReductionKg: kg, CostBasis: "not_estimated", Status: Open, Confidence: confidence,
		Compliance:  ComplianceCheck{Residency: "passed", CheckedAt: in.Now},
		Fingerprint: rightsizingFingerprint(in.ProjectID, f),
		Details:     map[string]string{"resource_id": f.ResourceID, "current_type": f.CurrentType},
	}
	if f.TargetType != "" {
		r.Details["action"], r.Details["target_type"] = "modify", f.TargetType
	} else {
		r.Details["action"] = "terminate"
	}
	if f.Currency == "USD" && f.EstimatedMonthlySavings > 0 {
		impact := -math.Round(f.EstimatedMonthlySavings*100) / 100 // negative = saving
		r.CostImpact, r.CostBasis = &impact, "provider_estimate"
	}
	return r
}

// rightsizingFingerprint identifies a finding: the same instance with a different target is a new finding,
// a decided finding is never recreated.
func rightsizingFingerprint(project string, f RightsizingFinding) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("rightsizing|%s|%s|%s|%s|%s", project, f.Region, f.ResourceID, f.CurrentType, f.TargetType)))
	return hex.EncodeToString(h[:])
}
