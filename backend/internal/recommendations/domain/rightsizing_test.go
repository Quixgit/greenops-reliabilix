package domain

import (
	"math"
	"testing"
	"time"
)

var testModel = EnergyModel{KWhPerVCPUHour: 0.002, KWhPerGBHour: 0.0004, PUE: 1.1}

func rsInput(fs ...RightsizingFinding) RightsizingInput {
	return RightsizingInput{ProjectID: "p1", Findings: fs, Intensity: map[string]float64{"eu-central-1": 400}, Model: testModel,
		Provider: "aws", Now: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
}

func TestModifyCarbonEffectFollowsTheShapes(t *testing.T) {
	res := GenerateRightsizing(rsInput(RightsizingFinding{ResourceID: "i-1", Region: "eu-central-1",
		CurrentType: "m5.2xlarge", TargetType: "m5.large", Current: Shape{8, 32}, Target: Shape{2, 8},
		EstimatedMonthlySavings: 120.456, Currency: "USD"}))
	if len(res.Recommendations) != 1 {
		t.Fatalf("got %d recommendations, skipped %v", len(res.Recommendations), res.Skipped)
	}
	r := res.Recommendations[0]
	before := 730 * (8*0.002 + 32*0.0004) * 1.1
	after := 730 * (2*0.002 + 8*0.0004) * 1.1
	wantKg := math.Round((before-after)*400/1000*10) / 10
	if r.Type != Rightsizing || r.CarbonReductionKg != wantKg || r.CarbonReductionPct != 75 {
		t.Errorf("effect wrong: kg %v (want %v), pct %v", r.CarbonReductionKg, wantKg, r.CarbonReductionPct)
	}
	if r.CostImpact == nil || *r.CostImpact != -120.46 || r.CostBasis != "provider_estimate" {
		t.Errorf("cost: %v %q (a saving is negative)", r.CostImpact, r.CostBasis)
	}
	if !r.ComplianceOK() || r.CurrentRegion != r.RecommendedRegion || r.Status != Open || r.Fingerprint == "" {
		t.Errorf("record invariants broken: %+v", r)
	}
}

func TestTerminateSavesAllEnergyOfTheInstance(t *testing.T) {
	res := GenerateRightsizing(rsInput(RightsizingFinding{ResourceID: "i-2", Region: "eu-central-1", CurrentType: "m5.large", Current: Shape{2, 8}}))
	if len(res.Recommendations) != 1 || res.Recommendations[0].CarbonReductionPct != 100 {
		t.Fatalf("terminate must remove 100%% of the instance: %+v / %v", res.Recommendations, res.Skipped)
	}
	if res.Recommendations[0].CostImpact != nil || res.Recommendations[0].CostBasis != "not_estimated" {
		t.Errorf("no provider estimate means cost is not estimated: %+v", res.Recommendations[0])
	}
}

func TestSkipsAreExplicitNotGuessed(t *testing.T) {
	model := testModel
	model.ShapeOf = func(it string) (Shape, bool) {
		if it == "m5.large" {
			return Shape{2, 8}, true
		}
		return Shape{}, false
	}
	in := rsInput(
		RightsizingFinding{ResourceID: "unknown-shape", Region: "eu-central-1", CurrentType: "p4d.24xlarge"},
		RightsizingFinding{ResourceID: "no-grid", Region: "mars-1", CurrentType: "m5.large"},
		RightsizingFinding{ResourceID: "same-size", Region: "eu-central-1", CurrentType: "m5.large", TargetType: "m5.large"},
		RightsizingFinding{ResourceID: "tiny", Region: "eu-central-1", CurrentType: "m5.large", TargetType: "m5.large", Current: Shape{2, 8}, Target: Shape{2, 7.9}},
	)
	in.Model = model
	res := GenerateRightsizing(in)
	if len(res.Recommendations) != 0 {
		t.Fatalf("nothing should be produced: %+v", res.Recommendations)
	}
	if res.Skipped[SkipNoShape] != 1 || res.Skipped[SkipNoIntensity] != 1 || res.Skipped[SkipNoReduction] != 1 || res.Skipped[SkipTooSmall] != 1 {
		t.Errorf("skip reasons wrong: %v", res.Skipped)
	}
}

func TestNonUSDSavingsAreNotReportedAsCost(t *testing.T) {
	res := GenerateRightsizing(rsInput(RightsizingFinding{ResourceID: "i-3", Region: "eu-central-1", CurrentType: "m5.xlarge",
		Current: Shape{4, 16}, EstimatedMonthlySavings: 50, Currency: "EUR"}))
	if len(res.Recommendations) != 1 || res.Recommendations[0].CostImpact != nil {
		t.Fatalf("a non-USD estimate must not be shown as a USD figure: %+v", res.Recommendations)
	}
}

func TestFingerprintDistinguishesTargets(t *testing.T) {
	a := rightsizingFingerprint("p", RightsizingFinding{ResourceID: "i-1", Region: "r", CurrentType: "m5.xlarge", TargetType: "m5.large"})
	b := rightsizingFingerprint("p", RightsizingFinding{ResourceID: "i-1", Region: "r", CurrentType: "m5.xlarge", TargetType: "t3.large"})
	if a == b || a != rightsizingFingerprint("p", RightsizingFinding{ResourceID: "i-1", Region: "r", CurrentType: "m5.xlarge", TargetType: "m5.large"}) {
		t.Error("fingerprint must be stable and target-specific")
	}
}

func TestOrderedByCarbon(t *testing.T) {
	res := GenerateRightsizing(rsInput(
		RightsizingFinding{ResourceID: "small", Region: "eu-central-1", CurrentType: "m5.xlarge", Current: Shape{4, 16}},
		RightsizingFinding{ResourceID: "big", Region: "eu-central-1", CurrentType: "m5.4xlarge", Current: Shape{16, 64}},
	))
	if len(res.Recommendations) != 2 || res.Recommendations[0].CarbonReductionKg < res.Recommendations[1].CarbonReductionKg {
		t.Fatalf("not sorted by carbon saved: %+v", res.Recommendations)
	}
}
