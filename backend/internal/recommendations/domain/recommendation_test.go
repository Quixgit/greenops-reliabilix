package domain

import (
	"errors"
	"testing"
	"time"
)

func TestWorkflow(t *testing.T) {
	ok := ComplianceCheck{Residency: "passed"}
	r := &Recommendation{Type: RegionShift, Status: Open, Compliance: ok}
	if err := r.Transition(Applied); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("open -> applied must be rejected (human approval required)")
	}
	if err := r.Transition(Approved); err != nil {
		t.Fatal(err)
	}
	if err := r.Transition(Applied); err != nil {
		t.Fatal(err)
	}
	if err := r.Transition(Open); err == nil {
		t.Fatal("applied is terminal")
	}
	for _, c := range []ComplianceCheck{{Residency: "failed"}, {Residency: "unknown"}, {}} {
		blocked := &Recommendation{Type: RegionShift, Status: Open, Compliance: c}
		if err := blocked.Transition(Approved); err == nil {
			t.Errorf("region_shift approved with residency %q", c.Residency)
		}
	}
	d := &Recommendation{Type: RegionShift, Status: Open}
	if err := d.Transition(Dismissed); err != nil {
		t.Errorf("dismiss must always be possible from open: %v", err)
	}
}

var now = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

func input() RegionShiftInput {
	return RegionShiftInput{
		Workloads:  []Workload{{Provider: "aws", Service: "Amazon EC2", Region: "us-east-1", EnergyKWh: 1000, CarbonKg: 380, Cost: 3000}},
		WindowDays: 30, DataDays: 30, Now: now,
		Intensity:      map[string]float64{"us-east-1": 380, "us-west-2": 150, "eu-north-1": 28, "eu-central-1": 340},
		AllowedRegions: []string{"us-west-2", "us-east-1"},
	}
}

func TestRegionShiftRespectsResidencyAllowList(t *testing.T) {
	recs := GenerateRegionShift(input())
	if len(recs) != 1 {
		t.Fatalf("got %d recs, want 1", len(recs))
	}
	r := recs[0]
	if r.RecommendedRegion != "us-west-2" {
		t.Errorf("recommended %s: eu-north-1 is cleaner but NOT allowed", r.RecommendedRegion)
	}
	if r.CarbonReductionPct < 60 || r.CarbonReductionPct > 61 || r.CarbonReductionKg < 229 || r.CarbonReductionKg > 230 {
		t.Errorf("effect: %v%% / %v kg", r.CarbonReductionPct, r.CarbonReductionKg)
	}
	if r.CostImpact != nil || r.CostBasis != "not_estimated" {
		t.Errorf("cost must not be invented without a price index: %v %s", r.CostImpact, r.CostBasis)
	}
	if !r.ComplianceOK() || r.Compliance.Residency != "passed" || r.Status != Open || r.Confidence <= 0 || r.Confidence > 0.9 {
		t.Errorf("compliance/confidence/status: %+v", r)
	}
}

func TestRegionShiftNeverWithoutAllowList(t *testing.T) {
	in := input()
	in.AllowedRegions = nil
	if recs := GenerateRegionShift(in); len(recs) != 0 {
		t.Errorf("residency unknown must produce nothing, got %d", len(recs))
	}
}

func TestRegionShiftCostFromPriceIndex(t *testing.T) {
	in := input()
	in.PriceIndex = map[string]float64{"us-east-1": 1.0, "us-west-2": 1.02}
	r := GenerateRegionShift(in)[0]
	if r.CostImpact == nil || r.CostBasis != "region_price_index" || *r.CostImpact != 60 {
		t.Errorf("cost impact = %v (%s), want +60", r.CostImpact, r.CostBasis)
	}
}

func TestRegionShiftThresholdsAndScaling(t *testing.T) {
	in := input()
	in.Intensity["us-west-2"] = 360 // only ~5% cleaner: below the 10% minimum
	if recs := GenerateRegionShift(in); len(recs) != 0 {
		t.Error("marginal gain must not be recommended")
	}
	in = input()
	in.WindowDays, in.DataDays = 15, 15 // half a month observed: monthly effect doubles
	r := GenerateRegionShift(in)[0]
	if r.CarbonReductionKg < 459 || r.CarbonReductionKg > 460 {
		t.Errorf("not scaled to a month: %v", r.CarbonReductionKg)
	}
	if r.Confidence >= GenerateRegionShift(input())[0].Confidence {
		t.Error("less data must lower confidence")
	}
	in = input()
	in.Workloads[0].Region = "ap-unknown-1" // no intensity for the current region
	if recs := GenerateRegionShift(in); len(recs) != 0 {
		t.Error("unknown current intensity must not be guessed")
	}
}
