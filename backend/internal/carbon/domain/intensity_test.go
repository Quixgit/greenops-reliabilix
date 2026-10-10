package domain

import (
	"context"
	"math"
	"testing"
	"time"
)

type fakeReader struct {
	day    map[string][2]float64 // provider -> mean, n (for every day)
	month  map[string]float64    // provider -> published monthly figure
	latest map[string]struct {
		g  float64
		at time.Time
	}
}

func (f fakeReader) DayStats(_ context.Context, _, provider string, _, _ time.Time) (float64, int, error) {
	v := f.day[provider]
	return v[0], int(v[1]), nil
}
func (f fakeReader) Monthly(_ context.Context, _, provider string, _ time.Time) (float64, bool, error) {
	g, ok := f.month[provider]
	return g, ok, nil
}
func (f fakeReader) Latest(_ context.Context, _, provider string, before *time.Time) (float64, time.Time, bool, error) {
	l, ok := f.latest[provider]
	if !ok || (before != nil && l.at.After(*before)) {
		return 0, time.Time{}, false, nil
	}
	return l.g, l.at, true, nil
}

var d0 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func TestFirstTrustedSourceWithEnoughReadingsWins_NeverAveraged(t *testing.T) {
	p := Policy{Priority: []string{"electricitymaps", "ukci"}, MinDayReadings: 6}
	r := fakeReader{day: map[string][2]float64{"electricitymaps": {300, 24}, "ukci": {100, 24}}}
	got, ok, err := ResolveDay(context.Background(), r, p, "eu-west-2", d0)
	if err != nil || !ok || got.G != 300 || got.Source != "electricitymaps" || got.Basis != BasisDayMean {
		t.Fatalf("%+v %v %v", got, ok, err)
	}
	// a regional override puts the other source first, still no averaging
	p.Overrides = map[string]string{"eu-west-2": "ukci"}
	got, _, _ = ResolveDay(context.Background(), r, p, "eu-west-2", d0)
	if got.G != 100 || got.Source != "ukci" {
		t.Errorf("override ignored: %+v", got)
	}
}

func TestFallsThroughWhenTheFirstSourceHasTooFewReadings(t *testing.T) {
	p := Policy{Priority: []string{"electricitymaps", "ukci"}, MinDayReadings: 6}
	r := fakeReader{day: map[string][2]float64{"electricitymaps": {300, 2}, "ukci": {120, 20}}}
	got, _, _ := ResolveDay(context.Background(), r, p, "x", d0)
	if got.Source != "ukci" || got.Basis != BasisDayMean {
		t.Errorf("want the source with enough readings, got %+v", got)
	}
	// nobody has enough: the first source with any reading, flagged partial
	r = fakeReader{day: map[string][2]float64{"electricitymaps": {300, 2}, "ukci": {120, 3}}}
	got, _, _ = ResolveDay(context.Background(), r, p, "x", d0)
	if got.Source != "electricitymaps" || got.Basis != BasisPartialDay || got.G != 300 {
		t.Errorf("want partial from the first source, got %+v", got)
	}
}

func TestLatestBeforeRespectsAgeAndOverallIsFlagged(t *testing.T) {
	p := Policy{Priority: []string{"electricitymaps"}, MinDayReadings: 6, MaxAge: 72 * time.Hour}
	fresh := d0.Add(-10 * time.Hour)
	r := fakeReader{latest: map[string]struct {
		g  float64
		at time.Time
	}{"electricitymaps": {250, fresh}}}
	got, ok, _ := ResolveDay(context.Background(), r, p, "x", d0)
	if !ok || got.Basis != BasisLatestBefore || got.G != 250 {
		t.Errorf("%+v", got)
	}
	r.latest["electricitymaps"] = struct {
		g  float64
		at time.Time
	}{250, d0.Add(-30 * 24 * time.Hour)}
	got, ok, _ = ResolveDay(context.Background(), r, p, "x", d0)
	if !ok || got.Basis != BasisLatestOverall {
		t.Errorf("an old reading must be flagged as an estimate: %+v", got)
	}
	got, ok, _ = ResolveDay(context.Background(), fakeReader{}, p, "x", d0)
	if ok {
		t.Errorf("nothing stored must stay unknown: %+v", got)
	}
}

func TestHistoryChoosesOneSourcePerDayAndKeepsOthersApart(t *testing.T) {
	p := Policy{Priority: []string{"electricitymaps", "ukci"}, MinDayReadings: 6}
	rows := []HistoryRow{
		{Day: d0, Provider: "ukci", Mean: 100, Lo: 90, Hi: 110, N: 24},
		{Day: d0, Provider: "electricitymaps", Mean: 300, Lo: 250, Hi: 350, N: 24},
		{Day: d0.AddDate(0, 0, 1), Provider: "electricitymaps", Mean: 310, N: 2},
		{Day: d0.AddDate(0, 0, 2), Provider: "unlisted", Mean: 50, N: 24},
	}
	h := ResolveHistory(rows, nil, p, "x", d0, d0.AddDate(0, 0, 3))
	if len(h) != 2 {
		t.Fatalf("a day with only an untrusted source must not appear: %+v", h)
	}
	if h[0].Source != "electricitymaps" || h[0].G != 300 || h[0].Basis != BasisDayMean || len(h[0].Others) != 1 || math.Abs(h[0].Others[0].G-100) > 1e-9 {
		t.Errorf("day 1 = %+v", h[0])
	}
	if h[1].Basis != BasisPartialDay || h[1].Readings != 2 {
		t.Errorf("day 2 = %+v", h[1])
	}
}

func TestMonthlyFigureBeatsAThinDayAndIsLabelled(t *testing.T) {
	p := Policy{Priority: []string{"electricitymaps", "ember"}, MinDayReadings: 6}
	r := fakeReader{day: map[string][2]float64{"electricitymaps": {300, 2}}, month: map[string]float64{"ember": 180}}
	got, ok, _ := ResolveDay(context.Background(), r, p, "x", d0)
	if !ok || got.Source != "ember" || got.Basis != BasisMonthMean || got.G != 180 {
		t.Errorf("a month figure must beat two readings: %+v", got)
	}
	// a day with enough readings still wins over the month figure
	r.day["electricitymaps"] = [2]float64{300, 20}
	got, _, _ = ResolveDay(context.Background(), r, p, "x", d0)
	if got.Source != "electricitymaps" || got.Basis != BasisDayMean {
		t.Errorf("enough hourly readings must win: %+v", got)
	}
}

func TestHistoryFillsDaysWithoutReadingsFromTheMonthlyFigure(t *testing.T) {
	p := Policy{Priority: []string{"electricitymaps", "ember"}, MinDayReadings: 6}
	rows := []HistoryRow{{Day: d0, Provider: "electricitymaps", Mean: 300, Lo: 250, Hi: 350, N: 24}}
	monthly := []MonthlyRow{{Month: d0, Provider: "ember", G: 180}}
	h := ResolveHistory(rows, monthly, p, "x", d0, d0.AddDate(0, 0, 3))
	if len(h) != 3 || h[0].Source != "electricitymaps" || h[1].Basis != BasisMonthMean || h[1].Source != "ember" || h[2].G != 180 {
		t.Fatalf("%+v", h)
	}
}
