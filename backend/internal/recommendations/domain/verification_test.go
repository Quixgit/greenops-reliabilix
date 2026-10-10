package domain

import (
	"testing"
	"time"
)

func series(from time.Time, days int, cost, kg float64) []DayPoint {
	out := make([]DayPoint, days)
	for i := range out {
		out[i] = DayPoint{Day: from.AddDate(0, 0, i), Cost: cost, CarbonKg: kg}
	}
	return out
}

func applied(kgMonth float64, cost *float64) Recommendation {
	at := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	return Recommendation{Status: Applied, AppliedAt: &at, CarbonReductionKg: kgMonth, CostImpact: cost}
}

func TestWindows(t *testing.T) {
	at := time.Date(2026, 9, 1, 10, 30, 0, 0, time.UTC)
	bFrom, bTo, aFrom, aTo := Windows(at)
	if !bTo.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) || !bFrom.Equal(bTo.AddDate(0, 0, -14)) {
		t.Fatalf("baseline %v..%v", bFrom, bTo)
	}
	if !aFrom.Equal(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)) || !aTo.Equal(aFrom.AddDate(0, 0, 14)) {
		t.Fatalf("after %v..%v", aFrom, aTo)
	}
	if Due(at, aTo.Add(-time.Second)) || !Due(at, aTo) {
		t.Fatal("due exactly when the after window is complete")
	}
	if Expired(at, at.AddDate(0, 0, 60)) || !Expired(at, at.AddDate(0, 0, 61)) {
		t.Fatal("expiry after 60 days")
	}
}

func TestVerifyOutcomes(t *testing.T) {
	r := applied(30, nil) // expects 30 kg/month = 1 kg/day
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	base := series(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), 14, 10, 5)
	for _, tc := range []struct {
		name      string
		afterKg   float64
		wantOut   Outcome
		wantMonth float64
	}{
		{"realized", 4, Realized, 30},
		{"partial", 4.5, Partial, 15},
		{"not realized", 4.9, NotRealized, 3},
		{"got worse", 6, NotRealized, -30},
	} {
		v := Verify(r, base, series(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC), 14, 10, tc.afterKg), now)
		if v.Outcome != tc.wantOut || v.MeasuredCarbonKgMonth == nil || *v.MeasuredCarbonKgMonth != tc.wantMonth {
			t.Errorf("%s: outcome %s measured %v, want %s %v", tc.name, v.Outcome, v.MeasuredCarbonKgMonth, tc.wantOut, tc.wantMonth)
		}
	}
}

func TestVerifyNeverExtrapolatesMissingData(t *testing.T) {
	r := applied(30, nil)
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	base := series(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), 14, 10, 5)
	v := Verify(r, base, series(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC), 9, 10, 4), now)
	if v.Outcome != Inconclusive || v.MeasuredCarbonKgMonth != nil || v.RealizationPct != nil {
		t.Fatalf("9 days must be inconclusive without numbers: %+v", v)
	}
	if v := Verify(r, nil, nil, now); v.Outcome != Inconclusive {
		t.Fatal("no data at all is inconclusive")
	}
}

func TestVerifyCostIsSeparateFromCarbon(t *testing.T) {
	saving := -100.0 // forecast: saves 100 per month
	r := applied(30, &saving)
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	base := series(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), 14, 10, 5)
	after := series(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC), 14, 9, 4)
	v := Verify(r, base, after, now)
	if v.ExpectedCostMonth == nil || *v.ExpectedCostMonth != 100 {
		t.Fatalf("expected cost saving: %v", v.ExpectedCostMonth)
	}
	if v.MeasuredCostMonth == nil || *v.MeasuredCostMonth != 30 {
		t.Fatalf("measured cost: %v", v.MeasuredCostMonth)
	}
}

func TestVerifiedIsReachedOnlyFromApplied(t *testing.T) {
	for _, from := range []Status{Open, Approved, Dismissed} {
		r := &Recommendation{Status: from, Compliance: ComplianceCheck{Residency: "passed"}}
		if err := r.Transition(Verified); err == nil {
			t.Errorf("%s -> verified must be rejected", from)
		}
	}
	r := &Recommendation{Status: Applied}
	if err := r.Transition(Verified); err != nil {
		t.Fatal(err)
	}
	if err := r.Transition(Open); err == nil {
		t.Fatal("verified is terminal")
	}
}
