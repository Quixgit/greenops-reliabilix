package domain

import "time"

// Measurement windows of the impact verification (ADR-0025).
const (
	BaselineDays = 14 // days before the change that describe "before"
	LagDays      = 3  // days after the change that are skipped: billing exports arrive late and settle slowly
	AfterDays    = 14 // days after the lag that describe "after"
	MinDays      = 10 // fewer days of data in a window and the change cannot be measured
	GiveUpDays   = 60 // days after "applied" at which a still-unmeasurable change is closed as inconclusive
	monthDays    = 30.0
)

// VerificationMethod names the measurement so a result is never read as more than it is.
const VerificationMethod = "before_after_daily_mean"

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
