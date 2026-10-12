package domain

import (
	"math"
	"sort"
	"time"
)

// Commitments: what a steady part of the cloud bill would save if it were bought as a commitment (a Savings Plan, a
// reservation or a committed-use discount) instead of paid at on-demand prices (ADR-0051). This is a report; nothing is bought.
//
// The method: the spend that can be committed (compute and databases) is looked at day by day. The level it stays above on 80% of
// the days is the baseline: a commitment of at most that size is used almost every day. The share of the baseline that is committed
// is the coverage. A commitment pays its discounted price whether the capacity is used or not, so the saving falls with the
// utilization and turns into a loss under the break-even utilization (1 - discount).

// DayCost is the cost of one provider on one day, in the reporting currency.
type DayCost struct {
	Provider string
	Day      time.Time
	// Committable is the compute and database spend (what a commitment can cover); Effective is the same by the amortized price.
	Committable, Effective float64
}

// Defaults: indicative discounts of a commitment against on-demand prices, by cloud and term in years. They are planning
// figures from the public price lists' typical ranges, not a quote: every cloud's real rate depends on the family, region, payment
// option and the customer's agreement, so the page lets the customer replace them.
var DefaultDiscountPct = map[string]map[int]float64{
	"aws":   {1: 25, 3: 40},
	"azure": {1: 15, 3: 35},
	"gcp":   {1: 28, 3: 46},
}

// Window limits and the thresholds of the method.
const (
	MinDataDays        = 14   // fewer days with data and the baseline would be a guess
	baselinePercentile = 0.20 // the level reached or exceeded on 80% of the days
	DefaultCoverage    = 70.0
)

// Scenario is the net monthly result when only Utilization percent of the committed capacity is used.
type Scenario struct {
	UtilizationPct float64 `json:"utilization_pct"`
	MonthlyNet     float64 `json:"monthly_net"`
}

// DailyPoint is one day of the committable spend, for the chart.
type DailyPoint struct {
	Day  string  `json:"day"`
	Cost float64 `json:"cost"`
}

// ProviderCommitment is the result for one cloud.
type ProviderCommitment struct {
	Provider string `json:"provider"`
	// Status is "ok" or "not_enough_data" (fewer than MinDataDays days of spend in the window).
	Status   string `json:"status"`
	DataDays int    `json:"data_days"`

	MonthlyCommittable float64 `json:"monthly_committable"` // average monthly spend that a commitment could cover
	BaselineDaily      float64 `json:"baseline_daily"`      // reached or exceeded on 80% of the days
	MedianDaily        float64 `json:"median_daily"`
	PeakDaily          float64 `json:"peak_daily"`
	StableSharePct     float64 `json:"stable_share_pct"` // share of the committable spend that sits at or under the baseline

	DiscountPct          float64 `json:"discount_pct"`
	CoveredMonthly       float64 `json:"covered_monthly"`        // on-demand value of what the commitment covers, per month
	CommitmentPerHour    float64 `json:"commitment_per_hour"`    // what to commit, per hour, at the discounted price
	MonthlySaving        float64 `json:"monthly_saving"`         // at full utilization
	AnnualSaving         float64 `json:"annual_saving"`          // monthly saving x 12
	BreakEvenUtilization float64 `json:"break_even_utilization"` // percent of the commitment that must be used to not lose money

	Scenarios []Scenario   `json:"scenarios"`
	Daily     []DailyPoint `json:"daily"`
	// PossiblyCommitted: the amortized price differs from the billed one by more than 3%, so part of the spend may already be under
	// a commitment; the baseline then includes capacity that is already discounted.
	PossiblyCommitted bool `json:"possibly_committed"`

	// What the cloud already holds and what to do about it. ExistingSource is "read" (from the cloud), "entered" (typed by the customer) or
	// "" (not known: the figures above then assume nothing is committed, and the page says to check the console first).
	ExistingSource         string   `json:"existing_source"`
	ExistingCoveragePct    *float64 `json:"existing_coverage_pct"`    // of the eligible spend
	ExistingUtilizationPct *float64 `json:"existing_utilization_pct"` // of the commitment
	ExistingUnusedMonthly  *float64 `json:"existing_unused_monthly"`  // paid for and not used
	// ExistingShareOfBaselinePct is the existing coverage restated as a share of the steady baseline (what AdditionalCoveragePct is measured from).
	ExistingShareOfBaselinePct float64 `json:"existing_share_of_baseline_pct"`
	AdditionalCoveragePct      float64 `json:"additional_coverage_pct"` // what to add to reach the target; 0 when covered or underused
	Advice                     Advice  `json:"advice"`
}

// Commitments is the whole report.
type Commitments struct {
	Currency     string               `json:"currency"`
	From         string               `json:"from"`
	To           string               `json:"to"`
	LookbackDays int                  `json:"lookback_days"`
	TermYears    int                  `json:"term_years"`
	CoveragePct  float64              `json:"coverage_pct"`
	Providers    []ProviderCommitment `json:"providers"`
	TotalMonthly float64              `json:"total_monthly_saving"`
	TotalAnnual  float64              `json:"total_annual_saving"`
	Defaults     map[string]float64   `json:"default_discount_pct"` // for the term chosen
}

// Existing is what a cloud account already holds in Savings Plans or reservations, as read from the cloud (ADR-0054).
type Existing struct {
	Provider       string
	Kind           string
	UtilizationPct *float64
	CoveragePct    *float64
	Commitment     *float64
	Unused         *float64 // over the 30 days the reading looks at
}

// ManualExisting is what the customer typed for a cloud the platform cannot read (or to correct a reading).
type ManualExisting struct{ CoveragePct, UtilizationPct *float64 }

// Advice is what to do about commitments in one cloud.
type Advice string

const (
	AdviceBuy    Advice = "buy"    // steady spend is not yet covered up to the target: more commitment would save
	AdviceHold   Advice = "hold"   // the target is already covered
	AdviceReduce Advice = "reduce" // the existing commitment is not fully used: fix that before buying more
)

// UnderusedBelowPct is the utilization under which an existing commitment is waste and more is not recommended.
const UnderusedBelowPct = 90.0

// CommitmentParams are the choices behind a report.
type CommitmentParams struct {
	// Read and Manual describe the commitments already held; Manual wins over Read for its cloud.
	Read         []Existing
	Manual       map[string]ManualExisting
	LookbackDays int
	TermYears    int
	CoveragePct  float64
	// DiscountPct overrides the default discount per provider; absent means the default.
	DiscountPct map[string]float64
	From, To    time.Time
	Currency    string
}

// percentile is the nearest-rank percentile (p in [0,1]) of an ascending slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(p*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	return sorted[i]
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// BuildCommitments computes the report from the daily series. Days with no row for a provider are not counted as zero spend: a
// gap in the data must not lower the baseline.
func BuildCommitments(days []DayCost, p CommitmentParams) Commitments {
	out := Commitments{Currency: p.Currency, From: p.From.Format(time.DateOnly), To: p.To.AddDate(0, 0, -1).Format(time.DateOnly), LookbackDays: p.LookbackDays,
		TermYears: p.TermYears, CoveragePct: p.CoveragePct, Providers: []ProviderCommitment{}, Defaults: map[string]float64{}}
	for prov, byTerm := range DefaultDiscountPct {
		out.Defaults[prov] = byTerm[p.TermYears]
	}
	byProv := map[string][]DayCost{}
	for _, d := range days {
		byProv[d.Provider] = append(byProv[d.Provider], d)
	}
	names := make([]string, 0, len(byProv))
	for n := range byProv {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		pc := providerCommitment(name, byProv[name], p, out.Defaults)
		applyExisting(&pc, p)
		out.Providers = append(out.Providers, pc)
		out.TotalMonthly += pc.MonthlySaving
		out.TotalAnnual += pc.AnnualSaving
	}
	out.TotalMonthly, out.TotalAnnual = round2(out.TotalMonthly), round2(out.TotalAnnual)
	return out
}

func providerCommitment(name string, ds []DayCost, p CommitmentParams, defaults map[string]float64) ProviderCommitment {
	pc := ProviderCommitment{Provider: name, Status: "ok", Scenarios: []Scenario{}, Daily: []DailyPoint{}}
	sort.Slice(ds, func(i, j int) bool { return ds[i].Day.Before(ds[j].Day) })
	var vals []float64
	var sum, eff float64
	for _, d := range ds {
		if d.Committable <= 0 {
			continue // a day with nothing to commit is a gap or an idle day; either way it does not set a floor
		}
		vals = append(vals, d.Committable)
		sum += d.Committable
		eff += d.Effective
		pc.Daily = append(pc.Daily, DailyPoint{Day: d.Day.Format(time.DateOnly), Cost: round2(d.Committable)})
	}
	pc.DataDays = len(vals)
	disc, ok := p.DiscountPct[name]
	if !ok {
		disc = defaults[name]
	}
	pc.DiscountPct = disc
	pc.BreakEvenUtilization = round2(100 - disc)
	if pc.DataDays < MinDataDays {
		pc.Status = "not_enough_data"
		return pc
	}
	asc := append([]float64(nil), vals...)
	sort.Float64s(asc)
	pc.BaselineDaily = round2(percentile(asc, baselinePercentile))
	pc.MedianDaily = round2(percentile(asc, 0.5))
	pc.PeakDaily = round2(asc[len(asc)-1])
	pc.MonthlyCommittable = round2(sum / float64(pc.DataDays) * 30.4)
	var under float64
	for _, v := range vals {
		under += math.Min(v, pc.BaselineDaily)
	}
	if sum > 0 {
		pc.StableSharePct = round2(under / sum * 100)
	}
	coveredDaily := pc.BaselineDaily * p.CoveragePct / 100
	pc.CoveredMonthly = round2(coveredDaily * 30.4)
	pc.CommitmentPerHour = round2(coveredDaily * (1 - disc/100) / 24)
	pc.MonthlySaving = round2(pc.CoveredMonthly * disc / 100)
	pc.AnnualSaving = round2(pc.MonthlySaving * 12)
	for _, u := range []float64{100, 80, 60} {
		pc.Scenarios = append(pc.Scenarios, Scenario{UtilizationPct: u, MonthlyNet: round2(pc.CoveredMonthly * (u/100 - (1 - disc/100)))})
	}
	pc.PossiblyCommitted = sum > 0 && math.Abs(eff-sum)/sum > 0.03
	return pc
}

// existingFor merges the readings of one cloud: utilization weighted by the commitment (a large commitment counts more), coverage as the plain
// mean. Savings Plans are preferred over reservations because they cover compute across families; reservations are used when there are no plans.
func existingFor(provider string, read []Existing) (cov, util, unused *float64, ok bool) {
	for _, kind := range []string{"savings_plans", "reservations"} {
		var cs, us, w, un float64
		var nc, nu, nun int
		for _, e := range read {
			if e.Provider != provider || e.Kind != kind {
				continue
			}
			weight := 1.0
			if e.Commitment != nil && *e.Commitment > 0 {
				weight = *e.Commitment
			}
			if e.CoveragePct != nil {
				cs += *e.CoveragePct
				nc++
			}
			if e.UtilizationPct != nil {
				us += *e.UtilizationPct * weight
				w += weight
				nu++
			}
			if e.Unused != nil {
				un += *e.Unused
				nun++
			}
		}
		if nc+nu == 0 {
			continue
		}
		if nc > 0 {
			v := round2(cs / float64(nc))
			cov = &v
		}
		if nu > 0 && w > 0 {
			v := round2(us / w)
			util = &v
		}
		if nun > 0 {
			v := round2(un)
			unused = &v
		}
		return cov, util, unused, true
	}
	return nil, nil, nil, false
}

// applyExisting restates a cloud's saving for what it already holds. Not known: nothing changes except the advice, which is "buy" with a
// warning. Underused (below 90%): no purchase is recommended, and the unused commitment is shown as the first thing to fix. Otherwise only
// the part of the target the existing commitment does not cover is counted.
func applyExisting(pc *ProviderCommitment, p CommitmentParams) {
	if pc.Status != "ok" {
		return
	}
	cov, util, unused, ok := existingFor(pc.Provider, p.Read)
	pc.ExistingSource = "read"
	if m, has := p.Manual[pc.Provider]; has && (m.CoveragePct != nil || m.UtilizationPct != nil) {
		cov, util, ok = m.CoveragePct, m.UtilizationPct, true
		pc.ExistingSource = "entered"
		unused = nil
	}
	if !ok {
		pc.ExistingSource = ""
		pc.Advice = AdviceBuy
		pc.AdditionalCoveragePct = p.CoveragePct
		return
	}
	pc.ExistingCoveragePct, pc.ExistingUtilizationPct, pc.ExistingUnusedMonthly = cov, util, unused
	baselineMonthly := pc.BaselineDaily * 30.4
	if cov != nil && baselineMonthly > 0 {
		pc.ExistingShareOfBaselinePct = round2(math.Min(100, *cov*pc.MonthlyCommittable/baselineMonthly))
	}
	switch {
	case util != nil && *util < UnderusedBelowPct:
		pc.Advice = AdviceReduce
		pc.AdditionalCoveragePct = 0
	default:
		pc.AdditionalCoveragePct = round2(math.Max(0, p.CoveragePct-pc.ExistingShareOfBaselinePct))
		pc.Advice = AdviceBuy
		if pc.AdditionalCoveragePct == 0 {
			pc.Advice = AdviceHold
		}
	}
	// the saving and the commitment are for what would be added, not for what is already held
	add := pc.AdditionalCoveragePct
	pc.CoveredMonthly = round2(pc.BaselineDaily * add / 100 * 30.4)
	pc.CommitmentPerHour = round2(pc.BaselineDaily * add / 100 * (1 - pc.DiscountPct/100) / 24)
	pc.MonthlySaving = round2(pc.CoveredMonthly * pc.DiscountPct / 100)
	pc.AnnualSaving = round2(pc.MonthlySaving * 12)
	for i := range pc.Scenarios {
		u := pc.Scenarios[i].UtilizationPct
		pc.Scenarios[i].MonthlyNet = round2(pc.CoveredMonthly * (u/100 - (1 - pc.DiscountPct/100)))
	}
}
