package domain

import "testing"

func TestRegionCloud(t *testing.T) {
	for id, want := range map[string]string{"eu-central-1": "aws", "us-gov-west-1": "aws", "europe-west9": "gcp", "us-central1": "gcp", "canadacentral": "azure", "eastus2": "azure", "global": "azure", "": ""} {
		if got := RegionCloud(id); got != want {
			t.Errorf("RegionCloud(%q) = %q, want %q", id, got, want)
		}
	}
}

// An Azure workload is never told to move to an AWS region, however clean it is.
func TestRegionShiftStaysInTheWorkloadsCloud(t *testing.T) {
	recs := GenerateRegionShift(RegionShiftInput{
		Workloads:  []Workload{{Provider: "azure", Service: "Virtual Machines", Region: "eastus2", EnergyKWh: 100, CarbonKg: 40, Cost: 100}},
		WindowDays: 30, DataDays: 30,
		Intensity:      map[string]float64{"eastus2": 400, "eu-north-1": 20, "canadaeast": 40},
		AllowedRegions: []string{"eu-north-1", "canadaeast"},
	})
	if len(recs) != 1 || recs[0].RecommendedRegion != "canadaeast" {
		t.Fatalf("want a move to canadaeast, got %+v", recs)
	}
	none := GenerateRegionShift(RegionShiftInput{
		Workloads: []Workload{{Provider: "azure", Service: "Virtual Machines", Region: "eastus2", CarbonKg: 40}}, WindowDays: 30, DataDays: 30,
		Intensity: map[string]float64{"eastus2": 400, "eu-north-1": 20}, AllowedRegions: []string{"eu-north-1"},
	})
	if len(none) != 0 {
		t.Fatalf("another cloud's region must not be proposed: %+v", none)
	}
}
