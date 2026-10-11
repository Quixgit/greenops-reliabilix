package hydroquebec

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAddWeightsTheSourcesAndSkipsEmptyHours(t *testing.T) {
	by := map[time.Time]float64{}
	f := func(v float64) *float64 { return &v }
	// 90 GW-ish mix scaled: 17000 hydro (24), 2000 wind (11), 600 other (230), 0 solar, 400 thermal (650)
	add(by, time.Date(2026, 10, 11, 0, 30, 0, 0, time.UTC), f(17000), f(2000), f(600), f(0), f(400))
	want := (17000*24 + 2000*11 + 600*230 + 400*650) / 20000.0
	got := by[time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)]
	if math.Abs(got-want) > 0.01 {
		t.Errorf("intensity %.2f, want %.2f", got, want)
	}
	add(by, time.Date(2026, 10, 11, 23, 30, 0, 0, time.UTC), nil, nil, nil, nil, nil) // a future hour, listed empty
	if len(by) != 1 {
		t.Errorf("an hour without generation has no intensity: %v", by)
	}
}

func TestHourlyUsesTheLiveDatasetForRecentHoursAndDropsFutureOnes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if !strings.Contains(r.URL.RawQuery, "order_by=date") {
			t.Errorf("the query must be ordered: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"date":"` + now.Add(-2*time.Hour).Add(30*time.Minute).Format(time.RFC3339) + `","valeurs_total":1000,"valeurs_hydraulique":1000,"valeurs_eolien":0,"valeurs_autres":0,"valeurs_solaire":0,"valeurs_thermique":0},
		{"date":"` + now.Add(5*time.Hour).Format(time.RFC3339) + `","valeurs_total":0,"valeurs_hydraulique":null,"valeurs_eolien":null,"valeurs_autres":null,"valeurs_solaire":null,"valeurs_thermique":null}]`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	got, err := c.Hourly(context.Background(), now.Add(-24*time.Hour), now.Add(24*time.Hour))
	if err != nil || len(got) != 1 || got[0].G != 24 {
		t.Fatalf("want the one real hour at 24 g/kWh (pure hydro), got %v %v", got, err)
	}
	if len(paths) != 1 || !strings.Contains(paths[0], "/production-electricite-quebec/") {
		t.Errorf("recent hours come from the live dataset only: %v", paths)
	}
}

func TestHourlyAsksTheHistoryForOldHours(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte(`[{"date":"2025-06-01T04:00:00+00:00","hydraulique":900,"eolien":100,"autres":0,"solaire":0,"thermique":0,"total":1000}]`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	got, err := c.Hourly(context.Background(), from, from.Add(48*time.Hour))
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	if len(paths) != 1 || !strings.Contains(paths[0], "/historique-production-electricite-quebec/") {
		t.Errorf("old hours come from the history: %v", paths)
	}
}
