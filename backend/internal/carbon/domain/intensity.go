package domain

import (
	"context"
	"sort"
	"time"
)

// Intensity measures. They are different quantities (the average emissions of all generation versus the emissions of the
// generator that would change at the margin) and are stored and read separately: they are never averaged together.
const (
	MeasureAverage  = "average"
	MeasureMarginal = "marginal"
)

// Basis says how the intensity of a calculation row was obtained, from the most to the least precise.
const (
	BasisDayMean       = "day_mean"         // mean of at least MinDayReadings readings of that day
	BasisPartialDay    = "partial_day_mean" // mean of fewer readings than the policy wants: usable, lower confidence
	BasisLatestBefore  = "latest_before"    // the last reading before the day, not older than MaxAge
	BasisLatestOverall = "latest_overall"   // the latest reading that exists, however old: an estimate, not a measurement
	BasisLive          = "live_provider"    // asked of the provider because nothing was stored
)

// IntensityRef is an intensity together with where it came from.
type IntensityRef struct {
	G      float64
	Source string // provider id
	Basis  string
}

// Precise reports whether the basis is a mean of readings of the day itself.
func (r IntensityRef) Precise() bool { return r.Basis == BasisDayMean }

// Policy decides which source is trusted for a region. It never averages sources: the first source in order that has
// enough data wins.
type Policy struct {
	Priority       []string          // provider ids, most trusted first
	Overrides      map[string]string // region -> provider, tried before Priority
	MinDayReadings int               // readings a source needs inside a day for its mean to count as BasisDayMean
	MaxAge         time.Duration     // how old the last reading before a day may be for BasisLatestBefore
}

// Order is the sources to try for a region, without repeats.
func (p Policy) Order(region string) []string {
	out := make([]string, 0, len(p.Priority)+1)
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(p.Overrides[region])
	for _, s := range p.Priority {
		add(s)
	}
	return out
}

func (p Policy) minReadings() int {
	if p.MinDayReadings < 1 {
		return 1
	}
	return p.MinDayReadings
}

// IntensityReader is what resolution needs from storage.
type IntensityReader interface {
	// DayStats is the mean and count of one source's average-intensity readings in [from, to).
	DayStats(ctx context.Context, region, provider string, from, to time.Time) (mean float64, n int, err error)
	// Latest is one source's newest reading at or before `before` (nil: overall).
	Latest(ctx context.Context, region, provider string, before *time.Time) (g float64, at time.Time, ok bool, err error)
}

// ResolveDay picks the intensity a whole day of usage ran under. Order of preference, never mixing sources:
//  1. a source with at least MinDayReadings readings that day (its mean),
//  2. a source with some readings that day (its mean, flagged partial),
//  3. a source's last reading before the day's end that is not older than MaxAge,
//  4. a source's latest reading of all (flagged as an estimate).
//
// Within each step the sources are tried in policy order. ok=false: no stored reading of any trusted source.
func ResolveDay(ctx context.Context, r IntensityReader, p Policy, region string, day time.Time) (IntensityRef, bool, error) {
	end := day.Add(24 * time.Hour)
	order := p.Order(region)
	type partial struct {
		g      float64
		source string
	}
	var firstPartial *partial
	for _, src := range order {
		mean, n, err := r.DayStats(ctx, region, src, day, end)
		if err != nil {
			return IntensityRef{}, false, err
		}
		switch {
		case n >= p.minReadings():
			return IntensityRef{G: mean, Source: src, Basis: BasisDayMean}, true, nil
		case n > 0 && firstPartial == nil:
			firstPartial = &partial{mean, src}
		}
	}
	if firstPartial != nil {
		return IntensityRef{G: firstPartial.g, Source: firstPartial.source, Basis: BasisPartialDay}, true, nil
	}
	return resolveLatest(ctx, r, p, region, &end)
}

// ResolveInstant is the intensity at a moment (an hour of a Kubernetes workload): the newest reading before it.
func ResolveInstant(ctx context.Context, r IntensityReader, p Policy, region string, at time.Time) (IntensityRef, bool, error) {
	return resolveLatest(ctx, r, p, region, &at)
}

func resolveLatest(ctx context.Context, r IntensityReader, p Policy, region string, before *time.Time) (IntensityRef, bool, error) {
	order := p.Order(region)
	for _, src := range order {
		g, at, ok, err := r.Latest(ctx, region, src, before)
		if err != nil {
			return IntensityRef{}, false, err
		}
		if ok && (p.MaxAge <= 0 || before.Sub(at) <= p.MaxAge) {
			return IntensityRef{G: g, Source: src, Basis: BasisLatestBefore}, true, nil
		}
	}
	for _, src := range order {
		g, _, ok, err := r.Latest(ctx, region, src, nil)
		if err != nil {
			return IntensityRef{}, false, err
		}
		if ok {
			return IntensityRef{G: g, Source: src, Basis: BasisLatestOverall}, true, nil
		}
	}
	return IntensityRef{}, false, nil
}

// HistoryRow is one source's readings of one day, as stored.
type HistoryRow struct {
	Day      time.Time
	Provider string
	Mean     float64
	Lo, Hi   float64
	N        int
}

// HistoryPoint is the day's intensity chosen under the policy, with the sources that were not chosen kept visible.
type HistoryPoint struct {
	Day      time.Time `json:"day"`
	G        float64   `json:"g_per_kwh"`
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	Readings int       `json:"readings"`
	Source   string    `json:"source"`
	Basis    string    `json:"basis"`
	// Others are the means of the other sources of that day. They are shown beside the chosen value, never combined with it.
	Others []HistoryOther `json:"others"`
}

type HistoryOther struct {
	Source string  `json:"source"`
	G      float64 `json:"g_per_kwh"`
}

// ResolveHistory chooses, for each day that has readings, the source the policy trusts: the first in order with at least
// MinDayReadings, else the first with any reading. Days with no reading do not appear (a gap is shown as a gap).
func ResolveHistory(rows []HistoryRow, p Policy, region string) []HistoryPoint {
	byDay := map[time.Time][]HistoryRow{}
	var days []time.Time
	for _, r := range rows {
		d := r.Day.UTC()
		if _, ok := byDay[d]; !ok {
			days = append(days, d)
		}
		byDay[d] = append(byDay[d], r)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	rank := map[string]int{}
	for i, s := range p.Order(region) {
		rank[s] = i + 1
	}
	out := make([]HistoryPoint, 0, len(days))
	for _, d := range days {
		cands := byDay[d]
		sort.SliceStable(cands, func(i, j int) bool {
			ri, rj := rank[cands[i].Provider], rank[cands[j].Provider]
			if ri == 0 {
				ri = 1 << 20 // sources outside the policy sort last and are never chosen below
			}
			if rj == 0 {
				rj = 1 << 20
			}
			return ri < rj
		})
		var chosen *HistoryRow
		for i := range cands {
			if rank[cands[i].Provider] > 0 && cands[i].N >= p.minReadings() {
				chosen = &cands[i]
				break
			}
		}
		if chosen == nil {
			for i := range cands {
				if rank[cands[i].Provider] > 0 {
					chosen = &cands[i]
					break
				}
			}
		}
		if chosen == nil {
			continue
		}
		pt := HistoryPoint{Day: d, G: chosen.Mean, Min: chosen.Lo, Max: chosen.Hi, Readings: chosen.N, Source: chosen.Provider, Basis: BasisPartialDay, Others: []HistoryOther{}}
		if chosen.N >= p.minReadings() {
			pt.Basis = BasisDayMean
		}
		for _, c := range cands {
			if c.Provider != chosen.Provider {
				pt.Others = append(pt.Others, HistoryOther{Source: c.Provider, G: c.Mean})
			}
		}
		out = append(out, pt)
	}
	return out
}
