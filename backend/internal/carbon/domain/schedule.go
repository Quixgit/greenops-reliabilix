package domain

import (
	"errors"
	"sort"
	"time"
)

// ForecastPoint is the forecast average grid intensity at a time. Points may be half-hourly or hourly.
type ForecastPoint struct {
	At time.Time
	G  float64
}

// BestWindow is the advice for a flexible job: when to start it so that its average grid intensity is lowest.
type BestWindow struct {
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	AverageG     float64   `json:"average_g_per_kwh"`
	NowStart     time.Time `json:"now_start"`
	NowAverageG  float64   `json:"now_average_g_per_kwh"`
	SavingPct    float64   `json:"saving_pct"`
	PointsInSpan int       `json:"points"`
}

var ErrNoForecast = errors.New("not enough forecast data for the requested window")

// ChooseWindow finds, among start times on the hour from `from` to the latest start that still ends by `deadline`, the
// one whose `duration` has the lowest mean forecast intensity. A window is only considered when the forecast covers it
// without gaps larger than gapMax; nothing is extrapolated. `from` itself is the "run now" baseline.
//
// The figure is the forecast of *average* grid intensity (not marginal): it says when the grid as a whole is cleanest, not
// what one job's marginal effect is.
func ChooseWindow(points []ForecastPoint, from, deadline time.Time, duration time.Duration) (BestWindow, error) {
	if duration <= 0 || !deadline.After(from) || deadline.Sub(from) < duration {
		return BestWindow{}, ErrNoForecast
	}
	pts := append([]ForecastPoint(nil), points...)
	sort.Slice(pts, func(i, j int) bool { return pts[i].At.Before(pts[j].At) })
	const gapMax = 90 * time.Minute
	mean := func(start time.Time) (g float64, n int, ok bool) {
		end := start.Add(duration)
		var sum float64
		var last time.Time
		for _, p := range pts {
			if p.At.Before(start) || !p.At.Before(end) {
				continue
			}
			if n == 0 && p.At.Sub(start) > gapMax {
				return 0, 0, false
			}
			if n > 0 && p.At.Sub(last) > gapMax {
				return 0, 0, false
			}
			sum += p.G
			n++
			last = p.At
		}
		if n == 0 || end.Sub(last) > gapMax {
			return 0, 0, false
		}
		return sum / float64(n), n, true
	}
	start := from.UTC().Truncate(time.Hour)
	if start.Before(from) {
		start = start.Add(time.Hour)
	}
	nowAvg, _, nowOK := mean(from.UTC().Truncate(time.Hour))
	best := BestWindow{AverageG: -1}
	for s := start; !s.Add(duration).After(deadline); s = s.Add(time.Hour) {
		g, n, ok := mean(s)
		if !ok {
			continue
		}
		if best.AverageG < 0 || g < best.AverageG {
			best = BestWindow{Start: s, End: s.Add(duration), AverageG: g, PointsInSpan: n}
		}
	}
	if best.AverageG < 0 {
		return BestWindow{}, ErrNoForecast
	}
	if nowOK {
		best.NowStart, best.NowAverageG = from.UTC().Truncate(time.Hour), nowAvg
		if nowAvg > 0 && best.AverageG < nowAvg {
			best.SavingPct = (nowAvg - best.AverageG) / nowAvg * 100
		}
	}
	return best, nil
}
