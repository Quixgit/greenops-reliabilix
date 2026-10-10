package domain

import (
	"errors"
	"testing"
	"time"
)

func hourly(start time.Time, gs ...float64) []ForecastPoint {
	out := make([]ForecastPoint, 0, len(gs))
	for i, g := range gs {
		out = append(out, ForecastPoint{At: start.Add(time.Duration(i) * time.Hour), G: g})
	}
	return out
}

func TestChooseWindowFindsTheCleanestStretchAndTheSaving(t *testing.T) {
	t0 := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	// now is dirty (400), three hours later the grid is clean (100)
	pts := hourly(t0, 400, 380, 300, 150, 100, 100, 120, 300, 350)
	w, err := ChooseWindow(pts, t0, t0.Add(9*time.Hour), 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !w.Start.Equal(t0.Add(4*time.Hour)) || w.AverageG != 100 || w.NowAverageG != 390 {
		t.Errorf("%+v", w)
	}
	if w.SavingPct < 74 || w.SavingPct > 75 { // (390-100)/390
		t.Errorf("saving = %v", w.SavingPct)
	}
}

func TestChooseWindowNeverExtrapolatesAndRespectsTheDeadline(t *testing.T) {
	t0 := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	pts := hourly(t0, 400, 380, 300) // the forecast ends after three hours
	if _, err := ChooseWindow(pts, t0, t0.Add(8*time.Hour), 4*time.Hour); !errors.Is(err, ErrNoForecast) {
		t.Errorf("a window the forecast does not cover = %v", err)
	}
	// the cleanest hour lies after the deadline: it must not be chosen
	pts = hourly(t0, 400, 300, 200, 100)
	w, err := ChooseWindow(pts, t0, t0.Add(3*time.Hour), time.Hour)
	if err != nil || !w.Start.Equal(t0.Add(2*time.Hour)) || w.AverageG != 200 {
		t.Errorf("%+v %v", w, err)
	}
	if _, err := ChooseWindow(pts, t0, t0, time.Hour); !errors.Is(err, ErrNoForecast) {
		t.Errorf("an empty span = %v", err)
	}
}

func TestChooseWindowAveragesHalfHourlyPoints(t *testing.T) {
	t0 := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	var pts []ForecastPoint
	for i := 0; i < 8; i++ { // four hours of half-hours: 200,200,200,200,100,100,100,100
		g := 200.0
		if i >= 4 {
			g = 100
		}
		pts = append(pts, ForecastPoint{At: t0.Add(time.Duration(i) * 30 * time.Minute), G: g})
	}
	w, err := ChooseWindow(pts, t0, t0.Add(4*time.Hour), time.Hour)
	if err != nil || !w.Start.Equal(t0.Add(2*time.Hour)) || w.AverageG != 100 {
		t.Errorf("%+v %v", w, err)
	}
}
