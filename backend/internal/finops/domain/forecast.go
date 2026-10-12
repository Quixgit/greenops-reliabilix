package domain

import (
	"math"
	"sort"
	"time"
)

// Forecast: the cost expected by the end of a period, from the daily history (ADR-0053). It replaces "spent so far scaled to the whole
// period", which is wrong in two ways every bill shows: spend has a weekly rhythm (a month that ends on a weekend is cheaper than the
// run-rate says), and the last day or two are still being delivered by the cloud, so they look like a drop.
//
// The method, in plain steps:
//  1. Trailing days that look unfinished (under 70% of what their weekday usually costs) are not trusted; they are forecast like the days
//     to come, so the billing lag never reads as a saving.
//  2. The expected cost of a coming day is the median of the same weekday over the last six weeks, scaled by the recent trend (the mean of
//     the last 14 trusted days over the 14 before them, kept within 0.8-1.25 so one odd fortnight cannot run away).
//  3. The band is the 80% range from how far this same rule missed on the trusted days it could be tested on.
//
// With fewer than 14 trusted days there is no pattern to use, and the result says "linear" instead of pretending.

// DayPoint is the cost of one calendar day (UTC) in one currency.
type DayPoint struct {
	Day  time.Time `json:"day"`
	Cost float64   `json:"cost"`
}

// Forecast is the expected total of a period and how sure it is.
type Forecast struct {
	Expected float64 `json:"expected"`
	Low      float64 `json:"low"`  // 10% chance of ending below
	High     float64 `json:"high"` // 10% chance of ending above
	// Method is "seasonal" (weekday pattern) or "linear" (too little history).
	Method string `json:"method"`
	// Spent is what is booked in the period so far, trusted or not; TrustedThrough is the last day taken as complete.
	Spent          float64   `json:"spent"`
	TrustedThrough time.Time `json:"trusted_through"`
	// LagDays is how many trailing days were treated as unfinished.
	LagDays int `json:"lag_days"`
	// DaysLeft counts the days forecast: the unfinished ones, today and the rest of the period.
	DaysLeft int `json:"days_left"`
}

const (
	minTrustedDays  = 14
	weeksBack       = 6
	unfinishedBelow = 0.70
	maxLagDays      = 2
	z80             = 1.2816
)

func day(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if n := len(s); n%2 == 1 {
		return s[n/2]
	} else {
		return (s[n/2-1] + s[n/2]) / 2
	}
}

// weekdayLevel is the median of the same weekday among the last weeks of h before `before` (exclusive).
func weekdayLevel(h []DayPoint, wd time.Weekday, before time.Time) (float64, bool) {
	var v []float64
	for i := len(h) - 1; i >= 0 && len(v) < weeksBack; i-- {
		if h[i].Day.Before(before) && h[i].Day.Weekday() == wd {
			v = append(v, h[i].Cost)
		}
	}
	if len(v) < 2 {
		return 0, false
	}
	return median(v), true
}

// trend is the mean of the last 14 days of h over the 14 before them, within [0.8, 1.25]; 1 when there are not 28 days.
func trend(h []DayPoint) float64 {
	n := len(h)
	if n < 28 {
		return 1
	}
	var a, b float64
	for _, p := range h[n-14:] {
		a += p.Cost
	}
	for _, p := range h[n-28 : n-14] {
		b += p.Cost
	}
	if b <= 0 {
		return 1
	}
	return math.Min(1.25, math.Max(0.8, a/b))
}

// predict is the expected cost of day d from the history that ends before d.
func predict(h []DayPoint, d time.Time) (float64, bool) {
	lvl, ok := weekdayLevel(h, d.Weekday(), d)
	if !ok {
		return 0, false
	}
	var before []DayPoint
	for _, p := range h {
		if p.Day.Before(d) {
			before = append(before, p)
		}
	}
	return lvl * trend(before), true
}

// BuildForecast forecasts the total of [start, end) at time now from the daily history (any order, days without data are absent). Days
// before start in the history are used for the weekday pattern but not counted in the total.
func BuildForecast(history []DayPoint, start, end, now time.Time) Forecast {
	start, end, today := day(start), day(end), day(now)
	pts := make([]DayPoint, 0, len(history))
	for _, p := range history {
		if p.Day = day(p.Day); p.Day.Before(today) { // today is still being booked: never a trusted day
			pts = append(pts, p)
		}
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].Day.Before(pts[j].Day) })

	f := Forecast{Method: "seasonal"}
	for _, p := range pts {
		if !p.Day.Before(start) && p.Day.Before(end) {
			f.Spent += p.Cost
		}
	}
	// 1. trailing days that look unfinished
	trusted := pts
	for lag := 0; lag < maxLagDays && len(trusted) > minTrustedDays; lag++ {
		last := trusted[len(trusted)-1]
		lvl, ok := weekdayLevel(trusted[:len(trusted)-1], last.Day.Weekday(), last.Day)
		if !ok || last.Cost >= unfinishedBelow*lvl {
			break
		}
		trusted = trusted[:len(trusted)-1]
		f.LagDays++
	}
	if len(trusted) > 0 {
		f.TrustedThrough = trusted[len(trusted)-1].Day
	}
	first := start
	if len(trusted) > 0 && f.TrustedThrough.AddDate(0, 0, 1).After(first) {
		first = f.TrustedThrough.AddDate(0, 0, 1)
	}
	if first.Before(start) {
		first = start
	}
	var done float64 // trusted cost inside the period
	for _, p := range trusted {
		if !p.Day.Before(start) && p.Day.Before(end) {
			done += p.Cost
		}
	}

	if len(trusted) < minTrustedDays {
		f.Method = "linear"
		// a cloud connected mid-period is measured from its first day, not from the start of the period
		from := start
		for _, p := range pts {
			if !p.Day.Before(start) {
				if p.Day.After(from) {
					from = p.Day
				}
				break
			}
		}
		elapsed := math.Max(1, today.Sub(from).Hours()/24)
		total := end.Sub(start).Hours() / 24
		f.Expected = f.Spent * total / elapsed
		f.Low, f.High = f.Expected, f.Expected
		f.DaysLeft = int(math.Max(0, end.Sub(today).Hours()/24)) + 1
		return f
	}

	// 2. the days still to forecast
	var rest float64
	for d := first; d.Before(end); d = d.AddDate(0, 0, 1) {
		p, ok := predict(trusted, d)
		if !ok {
			p = median(costs(trusted[max(0, len(trusted)-7):]))
		}
		rest += p
		f.DaysLeft++
	}
	f.Expected = done + rest

	// 3. the band from how far the rule missed on the last trusted days
	var errs []float64
	for i := len(trusted) - 1; i >= 0 && len(errs) < 28; i-- {
		if p, ok := predict(trusted[:i], trusted[i].Day); ok {
			errs = append(errs, trusted[i].Cost-p)
		}
	}
	sd := stddev(errs)
	band := z80 * sd * math.Sqrt(float64(f.DaysLeft)) // misses of different days partly cancel: they add as independent errors
	f.Low, f.High = math.Max(done, f.Expected-band), f.Expected+band
	return f
}

func costs(p []DayPoint) []float64 {
	out := make([]float64, len(p))
	for i, x := range p {
		out[i] = x.Cost
	}
	return out
}

func stddev(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	var m float64
	for _, x := range v {
		m += x
	}
	m /= float64(len(v))
	var s float64
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)-1))
}

// ProviderDays is one cloud's daily cost in the reporting currency.
type ProviderDays struct {
	Provider string     `json:"provider"`
	Days     []DayPoint `json:"days"`
}

// CloudForecast is one cloud's part of the forecast.
type CloudForecast struct {
	Provider string  `json:"provider"`
	Expected float64 `json:"expected"`
	Spent    float64 `json:"spent"`
	Method   string  `json:"method"`
	// LastDay is the newest day with cost; StaleDays is how many whole days before today it ended without data (0 = up to yesterday).
	LastDay   time.Time `json:"last_day"`
	StaleDays int       `json:"stale_days"`
}

// BuildForecastByProvider forecasts every cloud on its own history and adds them up. The bill of two clouds is two rhythms and two lags: one
// cloud that has not delivered its last days is forecast like the days to come, while the other keeps its booked figures. The band adds the
// clouds' misses as independent errors.
func BuildForecastByProvider(series []ProviderDays, start, end, now time.Time) (Forecast, []CloudForecast) {
	total := Forecast{Method: "seasonal"}
	var parts []CloudForecast
	var varLow, varHigh float64
	today := day(now)
	for _, s := range series {
		if len(s.Days) == 0 {
			continue
		}
		f := BuildForecast(s.Days, start, end, now)
		last := day(s.Days[0].Day)
		for _, p := range s.Days {
			if d := day(p.Day); d.After(last) {
				last = d
			}
		}
		stale := int(today.Sub(last).Hours()/24) - 1
		if stale < 0 {
			stale = 0
		}
		parts = append(parts, CloudForecast{Provider: s.Provider, Expected: f.Expected, Spent: f.Spent, Method: f.Method, LastDay: last, StaleDays: stale})
		total.Expected += f.Expected
		total.Spent += f.Spent
		varLow += (f.Expected - f.Low) * (f.Expected - f.Low)
		varHigh += (f.High - f.Expected) * (f.High - f.Expected)
		if f.Method != "seasonal" {
			total.Method = "linear"
		}
		if f.LagDays > total.LagDays {
			total.LagDays = f.LagDays
		}
		if f.DaysLeft > total.DaysLeft {
			total.DaysLeft = f.DaysLeft
		}
		if total.TrustedThrough.IsZero() || f.TrustedThrough.Before(total.TrustedThrough) {
			total.TrustedThrough = f.TrustedThrough
		}
	}
	total.Low = math.Max(total.Spent, total.Expected-math.Sqrt(varLow))
	total.High = total.Expected + math.Sqrt(varHigh)
	sort.Slice(parts, func(i, j int) bool { return parts[i].Expected > parts[j].Expected })
	return total, parts
}
