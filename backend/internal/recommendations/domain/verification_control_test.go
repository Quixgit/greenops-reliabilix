package domain

import (
	"math"
	"testing"
	"time"
)

func days(from time.Time, n int, cost, kg float64) []DayPoint {
	out := make([]DayPoint, n)
	for i := range out {
		out[i] = DayPoint{Day: from.AddDate(0, 0, i), Cost: cost, CarbonKg: kg}
	}
	return out
}

func appliedRec(carbon float64) Recommendation {
	at := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	c := -100.0
	return Recommendation{CarbonReductionKg: carbon, CostImpact: &c, AppliedAt: &at}
}

var t0 = time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)

// The whole workload grows 20% between the windows. The service itself went from 10 to 9 a day: the raw reading says it saved 30 a month
// and carbon 3, but compared with how the rest grew it should have cost 12, so the real saving is 90 a month.
func TestControlGroupRemovesTheWorkloadTrend(t *testing.T) {
	r := appliedRec(9)
	bF, _, aF, _ := Windows(*r.AppliedAt)
	before, after := days(bF, 14, 10, 1), days(aF, 14, 9, 0.9)
	ctrlB, ctrlA := days(bF, 14, 100, 10), days(aF, 14, 120, 12)
	v := VerifyControlled(r, before, after, ctrlB, ctrlA, r.AppliedAt.AddDate(0, 0, 40))
	if v.Method != VerificationMethodControlled || v.AdjustedCostMonth == nil {
		t.Fatalf("control not used: %+v", v)
	}
	if math.Abs(*v.MeasuredCostMonth-30) > 0.01 || math.Abs(*v.AdjustedCostMonth-90) > 0.01 {
		t.Errorf("raw %v adjusted %v, want 30 and 90", *v.MeasuredCostMonth, *v.AdjustedCostMonth)
	}
	if math.Abs(*v.ControlGrowthPct-20) > 0.01 || v.ControlDays != 14 {
		t.Errorf("control growth %v days %d", *v.ControlGrowthPct, v.ControlDays)
	}
	// adjusted carbon: 1*1.2 - 0.9 = 0.3 a day = 9 a month = 100% of the expected 9
	if v.RealizationPct == nil || math.Abs(*v.RealizationPct-100) > 0.1 || v.Outcome != Realized {
		t.Errorf("outcome %v realization %v", v.Outcome, v.RealizationPct)
	}
}

// A growing workload can hide a real saving; a shrinking one can fake it. The raw reading says "realized" while the control says nothing happened.
func TestAShrinkingWorkloadIsNotCreditedAsASaving(t *testing.T) {
	r := appliedRec(9)
	bF, _, aF, _ := Windows(*r.AppliedAt)
	before, after := days(bF, 14, 10, 1), days(aF, 14, 7, 0.7) // fell 30%
	ctrlB, ctrlA := days(bF, 14, 100, 10), days(aF, 14, 70, 7) // everything fell 30%
	v := VerifyControlled(r, before, after, ctrlB, ctrlA, r.AppliedAt.AddDate(0, 0, 40))
	if math.Abs(*v.AdjustedCarbonKgMonth) > 0.01 || v.Outcome != NotRealized {
		t.Fatalf("a drop everyone had is no saving: %+v", v)
	}
}

func TestWithoutAUsableControlTheRawResultStandsAndSaysWhy(t *testing.T) {
	r := appliedRec(9)
	bF, _, aF, _ := Windows(*r.AppliedAt)
	before, after := days(bF, 14, 10, 1), days(aF, 14, 9, 0.9)
	now := r.AppliedAt.AddDate(0, 0, 40)
	cases := map[string]struct{ cb, ca []DayPoint }{
		"too little data": {days(bF, 5, 100, 10), days(aF, 14, 100, 10)},
		"too small":       {days(bF, 14, 1, 0.1), days(aF, 14, 1, 0.1)},
		"moved itself":    {days(bF, 14, 100, 10), days(aF, 14, 300, 30)},
		"nothing before":  {days(bF, 14, 0, 0), days(aF, 14, 100, 10)},
	}
	for name, c := range cases {
		v := VerifyControlled(r, before, after, c.cb, c.ca, now)
		if v.Method != VerificationMethod || v.AdjustedCostMonth != nil || v.MeasuredCostMonth == nil {
			t.Errorf("%s: %+v", name, v)
		}
		if len(v.Note) < 60 {
			t.Errorf("%s: the note must say why: %q", name, v.Note)
		}
	}
}

func TestControlSeriesIsTheProjectLessTheService(t *testing.T) {
	d := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	total := []DayPoint{{Day: d, Cost: 100, CarbonKg: 10}, {Day: d.AddDate(0, 0, 1), Cost: 50, CarbonKg: 5}}
	target := []DayPoint{{Day: d, Cost: 30, CarbonKg: 4}}
	c := ControlSeries(total, target)
	if c[0].Cost != 70 || c[0].CarbonKg != 6 || c[1].Cost != 50 {
		t.Errorf("%+v", c)
	}
}
