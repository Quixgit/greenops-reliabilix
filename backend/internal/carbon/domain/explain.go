package domain

import (
	"context"
	"time"
)

// Verdicts of one source in an explanation: what the policy did with it for that region and day.
const (
	VerdictChosen    = "chosen"    // the day's intensity comes from this source
	VerdictOutranked = "outranked" // had enough readings too, but a source ahead of it in the order was used
	VerdictTooFew    = "too_few"   // has readings that day, fewer than the policy wants
	VerdictBackup    = "backup"    // no readings that day, but a monthly figure or a recent reading it could fall back on
	VerdictStale     = "stale"     // only readings older than the age the policy accepts
	VerdictNoData    = "no_data"   // nothing stored for the region
)

// SourceFacts is what one source has for a region and day, and what the policy made of it.
type SourceFacts struct {
	Source   string `json:"source"`
	Position int    `json:"position"` // 1-based place in the order for this region (a pinned source is first)
	Pinned   bool   `json:"pinned"`

	DayReadings int      `json:"day_readings"`
	DayMean     *float64 `json:"day_mean"`
	Monthly     *float64 `json:"monthly"`

	LatestBefore    *float64   `json:"latest_before"`
	LatestBeforeAt  *time.Time `json:"latest_before_at"`
	AgeHours        *float64   `json:"age_hours"` // of the latest reading before the end of the day
	MaxAgeHours     int        `json:"max_age_hours"`
	WithinMaxAge    bool       `json:"within_max_age"`
	LatestOverall   *float64   `json:"latest_overall"`
	LatestOverallAt *time.Time `json:"latest_overall_at"`

	Verdict string `json:"verdict"`
}

// Explanation answers "why did this region and day get this intensity": the order tried, the facts of every source in it,
// and the decision, which is the one ResolveDay makes (it is asked, not re-implemented).
type Explanation struct {
	Region         string        `json:"region"`
	Day            string        `json:"day"`
	MinDayReadings int           `json:"min_day_readings"`
	MaxAgeHours    int           `json:"max_age_hours"`
	Order          []string      `json:"order"`
	Sources        []SourceFacts `json:"sources"`
	Chosen         *IntensityRef `json:"chosen"` // nil: no stored reading of any trusted source
}

// Explain collects what each trusted source has for a region and day and the decision ResolveDay takes from it.
func Explain(ctx context.Context, r IntensityReader, p Policy, region string, day time.Time) (Explanation, error) {
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := day.Add(24 * time.Hour)
	order := p.Order(region)
	out := Explanation{Region: region, Day: day.Format(time.DateOnly), MinDayReadings: p.minReadings(), MaxAgeHours: int(p.MaxAge / time.Hour), Order: order, Sources: []SourceFacts{}}
	chosen, ok, err := ResolveDay(ctx, r, p, region, day)
	if err != nil {
		return out, err
	}
	if ok {
		out.Chosen = &chosen
	}
	for i, src := range order {
		f := SourceFacts{Source: src, Position: i + 1, Pinned: i == 0 && p.Overrides[region] == src, MaxAgeHours: int(p.maxAgeFor(src) / time.Hour)}
		mean, n, err := r.DayStats(ctx, region, src, day, end)
		if err != nil {
			return out, err
		}
		f.DayReadings = n
		if n > 0 {
			f.DayMean = &mean
		}
		if g, has, err := r.Monthly(ctx, region, src, day); err != nil {
			return out, err
		} else if has {
			f.Monthly = &g
		}
		if g, at, has, err := r.Latest(ctx, region, src, &end); err != nil {
			return out, err
		} else if has {
			age := end.Sub(at).Hours()
			f.LatestBefore, f.LatestBeforeAt, f.AgeHours = &g, &at, &age
			f.WithinMaxAge = p.maxAgeFor(src) <= 0 || end.Sub(at) <= p.maxAgeFor(src)
		}
		if g, at, has, err := r.Latest(ctx, region, src, nil); err != nil {
			return out, err
		} else if has {
			f.LatestOverall, f.LatestOverallAt = &g, &at
		}
		switch {
		case out.Chosen != nil && out.Chosen.Source == src:
			f.Verdict = VerdictChosen
		case n >= p.minReadings():
			f.Verdict = VerdictOutranked
		case n > 0:
			f.Verdict = VerdictTooFew
		case f.Monthly != nil || (f.LatestBefore != nil && f.WithinMaxAge):
			f.Verdict = VerdictBackup
		case f.LatestOverall != nil:
			f.Verdict = VerdictStale
		default:
			f.Verdict = VerdictNoData
		}
		out.Sources = append(out.Sources, f)
	}
	return out, nil
}
