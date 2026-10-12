package domain

import (
	"math"
	"testing"
	"time"
)

func utc(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// weekly builds a daily series from `from` to `to` inclusive: 100 on weekdays, 40 on Saturdays and Sundays.
func weekly(from, to time.Time, weekday, weekend float64) []DayPoint {
	var out []DayPoint
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		c := weekday
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			c = weekend
		}
		out = append(out, DayPoint{Day: d, Cost: c})
	}
	return out
}

func TestForecastFollowsTheWeekdayRhythm(t *testing.T) {
	// October 2026: the 1st is a Thursday. Now is Mon 12th noon; history from 1 Sep to Sun 11 Oct.
	h := weekly(utc(2026, 9, 1), utc(2026, 10, 11), 100, 40)
	f := BuildForecast(h, utc(2026, 10, 1), utc(2026, 11, 1), time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	// Whole October at this rhythm: 31 days, 8 weekend days (3,4,10,11,17,18,24,25,31 = 9) -> 22*100 + 9*40
	want := 22*100.0 + 9*40.0
	if f.Method != "seasonal" || math.Abs(f.Expected-want) > 1 {
		t.Fatalf("expected %v, got %+v", want, f)
	}
	// the straight scaling of spend so far overstates it: 11 days with 8 weekdays + ... is not the monthly mix
	linear := f.Spent * 31 / 12
	if math.Abs(linear-want) < 5 {
		t.Skip("the example does not separate the methods")
	}
	if f.Low > f.Expected || f.High < f.Expected {
		t.Errorf("band %v..%v around %v", f.Low, f.High, f.Expected)
	}
}

func TestAnUnfinishedLastDayIsForecastNotTrusted(t *testing.T) {
	h := weekly(utc(2026, 9, 1), utc(2026, 10, 10), 100, 40) // up to Saturday 10th
	h = append(h, DayPoint{Day: utc(2026, 10, 11), Cost: 5}) // Sunday: the cloud has delivered a sliver of 40
	f := BuildForecast(h, utc(2026, 10, 1), utc(2026, 11, 1), time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC))
	if f.LagDays != 1 || !f.TrustedThrough.Equal(utc(2026, 10, 10)) {
		t.Fatalf("lag %d trusted through %v", f.LagDays, f.TrustedThrough)
	}
	if want := 22*100.0 + 9*40; math.Abs(f.Expected-want) > 1 {
		t.Errorf("a late delivery must not read as a saving: %v, want %v", f.Expected, want)
	}
}

func TestTooLittleHistoryIsSaidToBeLinear(t *testing.T) {
	h := weekly(utc(2026, 10, 1), utc(2026, 10, 7), 100, 40)
	f := BuildForecast(h, utc(2026, 10, 1), utc(2026, 11, 1), time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC))
	if f.Method != "linear" || f.Low != f.Expected || f.High != f.Expected {
		t.Errorf("%+v", f)
	}
}

func TestTrendIsKeptWithinBounds(t *testing.T) {
	var h []DayPoint
	for d := utc(2026, 8, 1); d.Before(utc(2026, 10, 1)); d = d.AddDate(0, 0, 1) {
		c := 100.0
		if d.After(utc(2026, 9, 16)) {
			c = 1000 // a tenfold jump in the last fortnight
		}
		h = append(h, DayPoint{Day: d, Cost: c})
	}
	if tr := trend(h); tr != 1.25 {
		t.Errorf("trend %v must be capped", tr)
	}
}
