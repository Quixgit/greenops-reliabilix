package domain

import (
	"encoding/json"
	"strconv"
	"testing"
)

func torontoInput() RegionShiftInput {
	return RegionShiftInput{
		Workloads:  []Workload{{Provider: "azure", Service: "Azure App Service", Region: "canadacentral", EnergyKWh: 100, CarbonKg: 46, Cost: 200}},
		WindowDays: 30, DataDays: 30,
		// Paris and Oslo are cleaner than Quebec, but they are an ocean away from Toronto.
		Intensity:      map[string]float64{"canadacentral": 143, "canadaeast": 51, "francecentral": 39, "norwayeast": 25},
		AllowedRegions: []string{"canadaeast", "francecentral", "norwayeast"},
	}
}

// A workload is not told to cross an ocean for carbon alone: the cleanest region within reach is proposed, and the farther,
// cleaner ones are listed as options that did not fit.
func TestRegionShiftStaysWithinTheLatencyBudget(t *testing.T) {
	recs := GenerateRegionShift(torontoInput())
	if len(recs) != 1 || recs[0].RecommendedRegion != "canadaeast" {
		t.Fatalf("want the move to Quebec, next door, got %+v", recs)
	}
	r := recs[0]
	ms, _ := strconv.Atoi(r.Details["latency_ms"])
	if ms < 5 || ms > 20 || r.Details["latency_budget_ms"] != "60" {
		t.Errorf("latency of the move = %q ms (budget %q), want a few ms within 60", r.Details["latency_ms"], r.Details["latency_budget_ms"])
	}
	var opts []RegionOption
	if err := json.Unmarshal([]byte(r.Details["options"]), &opts); err != nil || len(opts) != 3 {
		t.Fatalf("options = %v, %v", r.Details["options"], err)
	}
	far := map[string]bool{}
	for _, o := range opts {
		if !o.WithinBudget {
			far[o.Region] = true
		}
		if o.Region == "canadaeast" && !o.Chosen {
			t.Error("the proposed region must be marked chosen")
		}
	}
	if !far["francecentral"] || !far["norwayeast"] {
		t.Errorf("the ocean-away regions must show as outside the budget: %v", far)
	}
}

func TestRegionShiftLatencyBudgetIsTheCallersChoice(t *testing.T) {
	in := torontoInput()
	in.MaxLatencyMs = 150 // a batch workload that does not care
	recs := GenerateRegionShift(in)
	if len(recs) != 1 || recs[0].RecommendedRegion != "norwayeast" {
		t.Fatalf("with a generous budget the cleanest allowed region wins, got %+v", recs)
	}
	in.MaxLatencyMs = 2 // nothing is that close
	if got := GenerateRegionShift(in); len(got) != 0 {
		t.Fatalf("nothing fits a 2 ms budget, got %+v", got)
	}
}

// Where the map does not know a region the distance cannot rule it out; it is still proposed, and says its latency is unknown.
func TestRegionShiftWithAnUnplacedRegionHasNoLatency(t *testing.T) {
	recs := GenerateRegionShift(RegionShiftInput{
		Workloads: []Workload{{Provider: "azure", Service: "Virtual Machines", Region: "eastus2", CarbonKg: 40}}, WindowDays: 30, DataDays: 30,
		Intensity: map[string]float64{"eastus2": 400, "mysteryregion": 40}, AllowedRegions: []string{"mysteryregion"},
	})
	if len(recs) != 1 || recs[0].Details["latency_ms"] != "" {
		t.Fatalf("want one move with no latency estimate: %+v", recs)
	}
}

func TestRegionOptionsAreTaggedOnEachAxis(t *testing.T) {
	in := torontoInput()
	in.AllowedRegions = []string{"canadaeast"}
	recs := GenerateRegionShift(in)
	var opts []RegionOption
	_ = json.Unmarshal([]byte(recs[0].Details["options"]), &opts)
	if len(opts) != 1 || len(opts[0].Tags) < 2 {
		t.Fatalf("the only option is both the cleanest and the closest: %+v", opts)
	}
}
