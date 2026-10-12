package domain

import (
	"strconv"
	"time"
)

// Measurement windows of the impact verification (ADR-0025).
const (
	BaselineDays = 14 // days before the change that describe "before"
	LagDays      = 3  // days after the change that are skipped: billing exports arrive late and settle slowly
	AfterDays    = 14 // days after the lag that describe "after"
	MinDays      = 10 // fewer days of data in a window and the change cannot be measured
	GiveUpDays   = 60 // days after "applied" at which a still-unmeasurable change is closed as inconclusive
	monthDays    = 30.0
)

// VerificationMethod names the measurement so a result is never read as more than it is. The controlled method is used when the rest of the
// project gives a usable comparison; the plain one when it does not.
const (
	VerificationMethod           = "before_after_daily_mean"
	VerificationMethodControlled = "before_after_vs_control"
)

// Control group thresholds (ADR-0054). The control is the project's other services; it is used only when it is big and steady enough to
// say something about the workload trend.
const (
	minControlShare = 0.30 // control daily cost at least 30% of the target's, or it is too small to be a reference
	minGrowth       = 0.5  // a control that halves or doubles between the windows moved for its own reasons, not the trend
	maxGrowth       = 2.0
)

type Outcome string

const (
	Realized     Outcome = "realized"     // at least 80 % of the expected carbon reduction was measured
	Partial      Outcome = "partial"      // 20 % up to 80 %
	NotRealized  Outcome = "not_realized" // below 20 % (or the carbon went up)
	Inconclusive Outcome = "inconclusive" // not enough data in one of the windows
)

// DayPoint is one day of the recommendation's scope (project and service), summed over regions so a region
// shift is measured as the total of the workload, not as a drop in the old region.
type DayPoint struct {
	Day      time.Time
	Cost     float64
	CarbonKg float64
}

// Verification is the stored measurement of one applied recommendation. Positive deltas are savings.
type Verification struct {
	Outcome               Outcome   `json:"outcome"`
	Method                string    `json:"method"`
	BaselineFrom          time.Time `json:"baseline_from"`
	BaselineTo            time.Time `json:"baseline_to"`
	AfterFrom             time.Time `json:"after_from"`
	AfterTo               time.Time `json:"after_to"`
	BaselineDays          int       `json:"baseline_days"`
	AfterDays             int       `json:"after_days"`
	MeasuredCarbonKgMonth *float64  `json:"measured_carbon_kg_month"`
	MeasuredCostMonth     *float64  `json:"measured_cost_month"`
	ExpectedCarbonKgMonth float64   `json:"expected_carbon_kg_month"`
	ExpectedCostMonth     *float64  `json:"expected_cost_month"` // positive = saved; nil = the forecast had no cost estimate
	RealizationPct        *float64  `json:"realization_pct"`
	// The measurement corrected for the trend of the rest of the project (the control group, ADR-0054): what the service would have cost
	// had it moved like the others, less what it did cost. Nil when the control was not usable; then the raw figures above are all there is.
	AdjustedCarbonKgMonth *float64  `json:"adjusted_carbon_kg_month"`
	AdjustedCostMonth     *float64  `json:"adjusted_cost_month"`
	ControlGrowthPct      *float64  `json:"control_growth_pct"` // how the control's daily cost moved between the windows
	ControlDays           int       `json:"control_days"`       // days of control data in the shorter of the two windows
	Note                  string    `json:"note"`
	VerifiedAt            time.Time `json:"verified_at"`
}

// Windows returns the baseline and after intervals [from, to) for a change applied at appliedAt.
func Windows(appliedAt time.Time) (bFrom, bTo, aFrom, aTo time.Time) {
	day := appliedAt.UTC().Truncate(24 * time.Hour)
	bTo = day
	bFrom = day.AddDate(0, 0, -BaselineDays)
	aFrom = day.AddDate(0, 0, 1+LagDays)
	aTo = aFrom.AddDate(0, 0, AfterDays)
	return
}

// Due reports whether the after window is complete and the change can be measured.
func Due(appliedAt, now time.Time) bool {
	_, _, _, aTo := Windows(appliedAt)
	return !now.Before(aTo)
}

// Expired reports whether the change has waited long enough for data that never came.
func Expired(appliedAt, now time.Time) bool {
	return now.After(appliedAt.AddDate(0, 0, GiveUpDays))
}

func daysWithData(ps []DayPoint) int {
	seen := map[time.Time]struct{}{}
	for _, p := range ps {
		seen[p.Day.UTC().Truncate(24*time.Hour)] = struct{}{}
	}
	return len(seen)
}

func meanPerDay(ps []DayPoint, n int) (cost, kg float64) {
	for _, p := range ps {
		cost += p.Cost
		kg += p.CarbonKg
	}
	return cost / float64(n), kg / float64(n)
}

// Verify compares the daily mean of the baseline window with the daily mean of the after window. It does not
// control for changes in workload: the result says what happened, not that the recommendation caused all of it.
// A window with fewer than MinDays days of data makes the result inconclusive; nothing is ever extrapolated.
func Verify(r Recommendation, baseline, after []DayPoint, now time.Time) Verification {
	appliedAt := now
	if r.AppliedAt != nil {
		appliedAt = *r.AppliedAt
	}
	bFrom, bTo, aFrom, aTo := Windows(appliedAt)
	v := Verification{
		Method: VerificationMethod, BaselineFrom: bFrom, BaselineTo: bTo, AfterFrom: aFrom, AfterTo: aTo,
		BaselineDays: daysWithData(baseline), AfterDays: daysWithData(after),
		ExpectedCarbonKgMonth: r.CarbonReductionKg, VerifiedAt: now,
	}
	if r.CostImpact != nil {
		saved := -*r.CostImpact // CostImpact is negative for a saving
		v.ExpectedCostMonth = &saved
	}
	if v.BaselineDays < MinDays || v.AfterDays < MinDays {
		v.Outcome = Inconclusive
		v.Note = "Not enough daily data around the change to measure it (need at least 10 days before and after)."
		return v
	}
	bCost, bKg := meanPerDay(baseline, v.BaselineDays)
	aCost, aKg := meanPerDay(after, v.AfterDays)
	carbon := round((bKg-aKg)*monthDays, 3)
	cost := round((bCost-aCost)*monthDays, 2)
	v.MeasuredCarbonKgMonth, v.MeasuredCostMonth = &carbon, &cost
	v.Note = "Measured as the change of the daily mean for the project and service, scaled to 30 days; changes in workload are not controlled for."

	switch {
	case r.CarbonReductionKg <= 0:
		v.Outcome = NotRealized
		if carbon > 0 {
			v.Outcome = Realized
		}
	default:
		pct := round(carbon/r.CarbonReductionKg*100, 1)
		v.RealizationPct = &pct
		switch {
		case pct >= 80:
			v.Outcome = Realized
		case pct >= 20:
			v.Outcome = Partial
		default:
			v.Outcome = NotRealized
		}
	}
	return v
}

// ControlSeries is the project without the recommendation's service: per day, the total less the service. A day that only the total has
// is the control's whole day.
func ControlSeries(total, target []DayPoint) []DayPoint {
	minus := map[time.Time]DayPoint{}
	for _, p := range target {
		minus[p.Day.UTC().Truncate(24*time.Hour)] = p
	}
	out := make([]DayPoint, 0, len(total))
	for _, p := range total {
		d := p.Day.UTC().Truncate(24 * time.Hour)
		t := minus[d]
		c := DayPoint{Day: d, Cost: p.Cost - t.Cost, CarbonKg: p.CarbonKg - t.CarbonKg}
		if c.Cost < 0 {
			c.Cost = 0
		}
		if c.CarbonKg < 0 {
			c.CarbonKg = 0
		}
		out = append(out, c)
	}
	return out
}

// VerifyControlled is Verify, corrected for the trend of the rest of the project. The two windows are 14 days each, so each holds two of
// every weekday and the weekly rhythm cancels; what is left to remove is growth or shrinkage of the whole workload. The service's expected
// cost after the change is its own before-mean times the control's growth; the saving is that less what was measured.
// The control must have at least MinDays days in both windows, a daily cost of at least 30% of the service's, and must not have moved by
// more than a factor of two (that is a change of its own, not a trend); otherwise the plain result stands and says why.
func VerifyControlled(r Recommendation, baseline, after, ctrlBefore, ctrlAfter []DayPoint, now time.Time) Verification {
	v := Verify(r, baseline, after, now)
	if v.Outcome == Inconclusive {
		return v
	}
	cb, ca := daysWithData(ctrlBefore), daysWithData(ctrlAfter)
	v.ControlDays = min(cb, ca)
	if cb < MinDays || ca < MinDays {
		v.Note += " No control group: the rest of the project has too little data."
		return v
	}
	tb, tbKg := meanPerDay(baseline, v.BaselineDays)
	ta, taKg := meanPerDay(after, v.AfterDays)
	xb, xbKg := meanPerDay(ctrlBefore, cb)
	xa, xaKg := meanPerDay(ctrlAfter, ca)
	switch {
	case xb <= 0 || xbKg <= 0:
		v.Note += " No control group: the rest of the project has no cost or carbon before the change."
		return v
	case xb < minControlShare*tb:
		v.Note += " No control group: the rest of the project is too small to say how the workload moved."
		return v
	}
	gCost, gKg := xa/xb, xaKg/xbKg
	if gCost < minGrowth || gCost > maxGrowth || gKg < minGrowth || gKg > maxGrowth {
		v.Note += " No control group: the rest of the project changed too much itself to be a reference."
		return v
	}
	carbon := round((tbKg*gKg-taKg)*monthDays, 3)
	cost := round((tb*gCost-ta)*monthDays, 2)
	growth := round((gCost-1)*100, 2)
	v.AdjustedCarbonKgMonth, v.AdjustedCostMonth, v.ControlGrowthPct = &carbon, &cost, &growth
	v.Method = VerificationMethodControlled
	v.Note = "Measured against the rest of the project: the service would have changed like the other services (" + signed(growth) +
		"% in daily cost) without the change; the saving is that expectation less what was measured, scaled to 30 days. " +
		"Changes made to the other services at the same time would blur it."
	v.RealizationPct, v.Outcome = nil, ""
	switch {
	case r.CarbonReductionKg <= 0:
		v.Outcome = NotRealized
		if carbon > 0 {
			v.Outcome = Realized
		}
	default:
		pct := round(carbon/r.CarbonReductionKg*100, 1)
		v.RealizationPct = &pct
		switch {
		case pct >= 80:
			v.Outcome = Realized
		case pct >= 20:
			v.Outcome = Partial
		default:
			v.Outcome = NotRealized
		}
	}
	return v
}

func signed(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if v > 0 {
		return "+" + s
	}
	return s
}
