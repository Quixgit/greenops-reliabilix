package domain

import (
	"errors"
	"math"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 10, 12, 30, 0, 0, time.UTC)

func ok() Sample {
	return Sample{Cluster: "prod-eu", Namespace: "shop", Workload: "checkout", Kind: "deployment",
		PeriodStart: time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC), EnergyKWh: 1.25, Region: "eu-central-1"}
}

func TestValidSampleIsAccepted(t *testing.T) {
	if err := ok().Validate(now); err != nil {
		t.Fatal(err)
	}
}

func TestSamplesThatCannotBeMeasurementsAreRefused(t *testing.T) {
	mut := func(f func(*Sample)) Sample { s := ok(); f(&s); return s }
	for name, s := range map[string]Sample{
		"bad name":          mut(func(s *Sample) { s.Workload = "../etc" }),
		"empty cluster":     mut(func(s *Sample) { s.Cluster = "" }),
		"bad kind":          mut(func(s *Sample) { s.Kind = "daemon" }),
		"bad region":        mut(func(s *Sample) { s.Region = "Frankfurt" }),
		"negative":          mut(func(s *Sample) { s.EnergyKWh = -1 }),
		"joules as kWh":     mut(func(s *Sample) { s.EnergyKWh = 4.5e6 }),
		"NaN":               mut(func(s *Sample) { s.EnergyKWh = math.NaN() }),
		"not on the hour":   mut(func(s *Sample) { s.PeriodStart = s.PeriodStart.Add(7 * time.Minute) }),
		"current hour":      mut(func(s *Sample) { s.PeriodStart = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }),
		"future":            mut(func(s *Sample) { s.PeriodStart = now.Add(48 * time.Hour).Truncate(time.Hour) }),
		"too old":           mut(func(s *Sample) { s.PeriodStart = now.AddDate(-2, 0, 0).Truncate(time.Hour) }),
		"negative cpu":      mut(func(s *Sample) { v := -1.0; s.CPUCoreSeconds = &v }),
		"memory in GB x1e9": mut(func(s *Sample) { v := 9e18; s.MemoryAvgBytes = &v }),
		"NaN request":       mut(func(s *Sample) { v := math.NaN(); s.CPURequestCores = &v }),
	} {
		if err := s.Validate(now); !errors.Is(err, ErrInvalidSample) {
			t.Errorf("%s must be refused, got %v", name, err)
		}
	}
}

func TestCarbonNeedsAGridReading(t *testing.T) {
	kg, okk := Carbon(2, 1.135, 400, true)
	if !okk || math.Abs(kg-0.908) > 1e-9 {
		t.Fatalf("%v %v", kg, okk)
	}
	if _, okk := Carbon(2, 1.135, 0, false); okk {
		t.Fatal("no intensity, no carbon: never a guess")
	}
}

func TestUsageFieldsAreOptionalAndUtilizationNeedsARequest(t *testing.T) {
	s := ok()
	cpu, mem := 1800.0, 2e9
	s.CPUCoreSeconds, s.MemoryAvgBytes = &cpu, &mem
	if err := s.Validate(now); err != nil {
		t.Fatalf("usage without requests is valid: %v", err)
	}
	if pct, okk := Utilization(1800, 2*3600); !okk || pct != 25 {
		t.Fatalf("1800 core-seconds of a 2-core request over an hour is 25%%: %v %v", pct, okk)
	}
	if _, okk := Utilization(1, 0); okk {
		t.Fatal("no request, no utilization")
	}
}
