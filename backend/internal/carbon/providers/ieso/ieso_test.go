package ieso

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func report(rows ...string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><Document xmlns="http://www.ieso.ca/schema"><DocBody><DeliveryYear>2026</DeliveryYear>` + strings.Join(rows, "") + `</DocBody></Document>`
}

func hour(day string, h int, nuclear, gas, hydro, wind float64) string {
	f := func(name string, v float64) string {
		return fmt.Sprintf("<FuelTotal><Fuel>%s</Fuel><EnergyValue><OutputQuality>0</OutputQuality><Output>%v</Output></EnergyValue></FuelTotal>", name, v)
	}
	return fmt.Sprintf("<DailyData><Day>%s</Day><HourlyData><Hour>%d</Hour>%s%s%s%s</HourlyData></DailyData>", day, h, f("NUCLEAR", nuclear), f("GAS", gas), f("HYDRO", hydro), f("WIND", wind))
}

func TestParseWeightsTheFuelsAndPlacesHourEndingInEST(t *testing.T) {
	// 3000 MW gas at 490, 7000 MW nuclear at 12: (3000*490 + 7000*12) / 10000 = 155.4
	rs, err := Parse(strings.NewReader(report(hour("2026-10-10", 1, 7000, 3000, 0, 0))))
	if err != nil || len(rs) != 1 {
		t.Fatalf("%v %v", rs, err)
	}
	if math.Abs(rs[0].G-155.4) > 0.01 {
		t.Errorf("intensity = %.2f, want 155.40", rs[0].G)
	}
	// hour 1 of 10 October is 00:00-01:00 EST, which is 05:00 UTC: reports have no daylight saving
	if want := time.Date(2026, 10, 10, 5, 0, 0, 0, time.UTC); !rs[0].At.Equal(want) {
		t.Errorf("hour 1 starts at %v, want %v", rs[0].At, want)
	}
	last, _ := Parse(strings.NewReader(report(hour("2026-10-10", 24, 1, 0, 0, 0))))
	if want := time.Date(2026, 10, 11, 4, 0, 0, 0, time.UTC); !last[0].At.Equal(want) {
		t.Errorf("hour 24 starts at %v, want %v", last[0].At, want)
	}
}

func TestParseSkipsHoursWithNoGenerationAndRefusesGarbage(t *testing.T) {
	rs, _ := Parse(strings.NewReader(report(hour("2026-10-10", 3, 0, 0, 0, 0), hour("2026-10-10", 4, 100, 0, 0, 0))))
	if len(rs) != 1 || rs[0].G != 12 {
		t.Errorf("an hour without generation has no intensity: %v", rs)
	}
	if _, err := Parse(strings.NewReader("not xml")); err == nil {
		t.Error("an unreadable report must be an error, not an empty answer")
	}
}

func TestHourlyAsksOneFilePerYearAndKeepsTheWindow(t *testing.T) {
	years = map[int]*cached{}
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		_, _ = w.Write([]byte(report(hour("2026-10-10", 1, 7000, 3000, 0, 0), hour("2026-10-10", 2, 7000, 3000, 0, 0), hour("2026-10-10", 3, 7000, 3000, 0, 0))))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	from := time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC) // 01:00 EST
	got, err := c.Hourly(context.Background(), from, from.Add(2*time.Hour))
	if err != nil || len(got) != 2 {
		t.Fatalf("want the two hours inside the window, got %v %v", got, err)
	}
	if _, err := c.Hourly(context.Background(), from, from.Add(2*time.Hour)); err != nil || len(asked) != 1 {
		t.Errorf("the year's file is kept for a while: asked %v", asked)
	}
	if !strings.HasSuffix(asked[0], "/PUB_GenOutputbyFuelHourly_2026.xml") {
		t.Errorf("asked %v", asked)
	}
}
