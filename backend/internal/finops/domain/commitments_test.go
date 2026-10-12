package domain

import (
	"math"
	"testing"
	"time"
)

func series(provider string, start time.Time, vals ...float64) []DayCost {
	var out []DayCost
	for i, v := range vals {
		out = append(out, DayCost{Provider: provider, Day: start.AddDate(0, 0, i), Committable: v, Effective: v})
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.011 }

func params() CommitmentParams {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return CommitmentParams{LookbackDays: 30, TermYears: 1, CoveragePct: 100, From: from, To: from.AddDate(0, 0, 30), Currency: "USD", DiscountPct: map[string]float64{"aws": 30}}
}

func TestBaselineIsWhatTheSpendStaysAboveOnMostDays(t *testing.T) {
	// 20 days at 100, 5 at 60, 5 at 300: the spend is at 100 or more on 25 of 30 days, so the floor reached on 80% of days is 100.
	v := append(append(make([]float64, 0, 30), repeat(100, 20)...), append(repeat(60, 5), repeat(300, 5)...)...)
	r := BuildCommitments(series("aws", params().From, v...), params())
	pc := r.Providers[0]
	if pc.Status != "ok" || pc.DataDays != 30 {
		t.Fatalf("%+v", pc)
	}
	if !near(pc.BaselineDaily, 100) {
		t.Errorf("baseline %v, want 100 (the 6th lowest of 30)", pc.BaselineDaily)
	}
	// 100% coverage, 30% discount: saving = 100*30.4*0.3 per month
	if !near(pc.MonthlySaving, 912) || !near(pc.AnnualSaving, 10944) {
		t.Errorf("saving %v / %v", pc.MonthlySaving, pc.AnnualSaving)
	}
	if !near(pc.CommitmentPerHour, 100*0.7/24) {
		t.Errorf("per hour %v", pc.CommitmentPerHour)
	}
}

func TestSavingTurnsIntoALossBelowBreakEven(t *testing.T) {
	r := BuildCommitments(series("aws", params().From, repeat(100, 30)...), params())
	pc := r.Providers[0]
	if !near(pc.BreakEvenUtilization, 70) {
		t.Errorf("break-even %v", pc.BreakEvenUtilization)
	}
	// covered 3040: full use +912, 80% use +(0.8-0.7)*3040=+304, 60% use -304
	want := []float64{912, 304, -304}
	for i, s := range pc.Scenarios {
		if !near(s.MonthlyNet, want[i]) {
			t.Errorf("utilization %v: %v, want %v", s.UtilizationPct, s.MonthlyNet, want[i])
		}
	}
}

func TestCoverageScalesTheCommitmentAndTooFewDaysIsNotGuessed(t *testing.T) {
	p := params()
	p.CoveragePct = 50
	r := BuildCommitments(series("aws", p.From, repeat(100, 30)...), p)
	if !near(r.Providers[0].MonthlySaving, 456) {
		t.Errorf("half coverage: %v", r.Providers[0].MonthlySaving)
	}
	short := BuildCommitments(series("gcp", p.From, repeat(100, MinDataDays-1)...), p)
	if short.Providers[0].Status != "not_enough_data" || short.Providers[0].MonthlySaving != 0 {
		t.Errorf("%+v", short.Providers[0])
	}
}

func TestDefaultDiscountAndDaysWithoutSpendDoNotLowerTheFloor(t *testing.T) {
	p := params()
	v := append(repeat(100, 20), repeat(0, 10)...) // ten idle or missing days
	r := BuildCommitments(series("gcp", p.From, v...), p)
	pc := r.Providers[0]
	if pc.DataDays != 20 || !near(pc.BaselineDaily, 100) {
		t.Errorf("%+v", pc)
	}
	if pc.DiscountPct != DefaultDiscountPct["gcp"][1] || r.Defaults["gcp"] != 28 {
		t.Errorf("default discount %v", pc.DiscountPct)
	}
}

func TestAmortizedPriceFarFromBilledFlagsAnExistingCommitment(t *testing.T) {
	p := params()
	d := series("aws", p.From, repeat(100, 30)...)
	for i := range d {
		d[i].Effective = 80
	}
	if !BuildCommitments(d, p).Providers[0].PossiblyCommitted {
		t.Error("effective 20% under billed must be flagged")
	}
}

func repeat(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}
