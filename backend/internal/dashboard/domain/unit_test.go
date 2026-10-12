package domain

import (
	"math"
	"testing"
)

func TestUnitEconomicsComparesTheTwoLatestComparableMonths(t *testing.T) {
	u := UnitEconomics{Projects: []UnitProject{{Months: []UnitMonth{
		{Month: "2026-08", Days: 31, Units: 1000, Cost: 100, CarbonG: 5000},
		{Month: "2026-09", Days: 30, Units: 1000, Cost: 80, CarbonG: 4000},
		{Month: "2026-10", Days: 3, Units: 10, Cost: 50, CarbonG: 900}, // too few reported days: not compared
	}}}}
	u.Finish()
	p := u.Projects[0]
	if p.CostChangePct == nil || math.Abs(*p.CostChangePct+20) > 1e-9 || math.Abs(*p.CarbonChangePct+20) > 1e-9 {
		t.Fatalf("change %v %v, want -20%% both", p.CostChangePct, p.CarbonChangePct)
	}
	if got := *p.Months[0].CostPerUnit; math.Abs(got-0.1) > 1e-9 {
		t.Errorf("cost per unit %v", got)
	}
}

func TestUnitEconomicsHasNoChangeWithoutAPair(t *testing.T) {
	u := UnitEconomics{Projects: []UnitProject{{Months: []UnitMonth{{Month: "2026-10", Days: 20, Units: 100, Cost: 10, CarbonG: 100}}}}}
	u.Finish()
	if u.Projects[0].CostChangePct != nil {
		t.Error("one month cannot change")
	}
	empty := UnitMonth{Days: 2}
	v := UnitEconomics{Projects: []UnitProject{{Months: []UnitMonth{empty}}}}
	v.Finish()
	if v.Projects[0].Months[0].CostPerUnit != nil {
		t.Error("no units: no per-unit figure")
	}
}
