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
		MaxLatencyMs:   80, // CloudPing measures 68 ms between us-east-1 and us-west-2: beyond the 60 ms default, so these tests widen it
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

func TestRegionShiftPriceIndexIsForEC2ComputeOnly(t *testing.T) {
	in := input()
	in.PriceIndex = map[string]float64{"us-east-1": 1.0, "us-west-2": 1.1}
	for _, svc := range []string{"Amazon Relational Database Service", "Amazon Simple Storage Service", "AWS Lambda"} {
		in.Workloads[0].Service = svc
		r := GenerateRegionShift(in)[0]
		if r.CostImpact != nil || r.CostBasis != "not_estimated" {
			t.Errorf("%s: an EC2 price basket must not price %s (%v %s)", svc, svc, r.CostImpact, r.CostBasis)
		}
	}
	in.Workloads[0].Service = "Amazon Elastic Compute Cloud - Compute"
	if r := GenerateRegionShift(in)[0]; r.CostImpact == nil || *r.CostImpact != 300 || r.CostBasis != "region_price_index" {
		t.Errorf("EC2 compute: want +300 from a 10%% dearer region, got %v %s", r.CostImpact, r.CostBasis)
	}
	in.Workloads[0].Provider = "gcp"
	// a GCP workload is not moved into AWS regions at all; and if it ever were, an AWS price index must not price it
	if rs := GenerateRegionShift(in); len(rs) > 0 && rs[0].CostImpact != nil {
		t.Error("an AWS price index must not price another cloud")
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

func TestManagedServicesAreMeasuredByTheirOwnIndex(t *testing.T) {
	in := input()
	in.Workloads[0].Provider = "azure"
	in.AllowedRegions = []string{"us-east-1", "us-west-2"}
	in.PriceIndex = map[string]float64{"us-east-1": 1.0, "us-west-2": 2.0} // the VM index must never price a database
	in.ServiceIndex = map[string]map[string]float64{"azure:postgresql": {"us-east-1": 1.0, "us-west-2": 1.1}}
	in.Workloads[0].Service = "Azure Database for PostgreSQL"
	// RegionCloud decides which regions are the workload's cloud; the test only needs the family to select the right index
	if idx := priceIndexFor(in, in.Workloads[0]); idx["us-west-2"] != 1.1 {
		t.Fatalf("PostgreSQL must use its own index, got %v", idx)
	}
	in.Workloads[0].Service = "Redis Cache"
	if idx := priceIndexFor(in, in.Workloads[0]); idx != nil {
		t.Fatalf("Redis has no index here, nothing must be priced: %v", idx)
	}
	in.Workloads[0].Service = "Virtual Machines"
	if idx := priceIndexFor(in, in.Workloads[0]); idx["us-west-2"] != 2.0 {
		t.Fatalf("a VM uses the VM index, got %v", idx)
	}
	in.Workloads[0].Service = "Azure Virtual Network"
	if priceIndexFor(in, in.Workloads[0]) != nil {
		t.Fatal("networking has no index")
	}
}

func TestManagedFamilyNamesTheIndexOfEachCloudsService(t *testing.T) {
	for _, c := range []struct{ provider, service, want string }{
		{"aws", "Amazon Relational Database Service", "aws:rds"}, {"aws", "Amazon ElastiCache", "aws:elasticache"},
		{"azure", "Azure Database for PostgreSQL", "azure:postgresql"}, {"azure", "Redis Cache", "azure:redis"},
		{"gcp", "Cloud SQL", "gcp:cloudsql"}, {"gcp", "Cloud Memorystore for Redis", "gcp:memorystore"},
		{"aws", "Amazon Simple Storage Service", ""}, {"gcp", "Cloud Storage", ""}, {"azure", "Virtual Network", ""},
	} {
		if got := ManagedFamily(c.provider, c.service); got != c.want {
			t.Errorf("%s %q = %q, want %q", c.provider, c.service, got, c.want)
		}
	}
}
