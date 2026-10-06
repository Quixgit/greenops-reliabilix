package domain

import (
	"context"
	"math"
	"testing"
	"time"
)

var d1 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func fixedIntensity(m map[string]float64) IntensityLookup {
	return func(_ context.Context, region string, _ time.Time) (float64, bool, error) {
		g, ok := m[region]
		return g, ok, nil
	}
}

func str(s string) *string { return &s }

func TestComputeMethodsAndSkips(t *testing.T) {
	rows := []UsageDay{
		{Provider: "aws", ServiceName: "EC2", ServiceCategory: "compute", RegionID: "eu-central-1", Currency: "USD", BilledCost: 100, Day: d1},
		{Provider: "k8s", ServiceName: "pods", ServiceCategory: "compute", RegionID: "eu-central-1", Currency: "USD", Day: d1,
			ConsumedUnit: str("vCPU-Hrs"), Quantity: 500, HasQuantity: true},
		{Provider: "aws", ServiceName: "Route 53", ServiceCategory: "networking", RegionID: "global", Currency: "USD", BilledCost: 1, Day: d1},
		{Provider: "aws", ServiceName: "S3", ServiceCategory: "storage", RegionID: "mars-1", Currency: "USD", BilledCost: 5, Day: d1},
		{Provider: "aws", ServiceName: "RDS", ServiceCategory: "database", RegionID: "eu-central-1", Currency: "EUR", BilledCost: 9, Day: d1},
	}
	out, skipped, err := Compute(context.Background(), rows, fixedIntensity(map[string]float64{"eu-central-1": 400}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("calculations = %d, want 2: %+v", len(out), out)
	}
	if out[0].Method != CostBased || out[1].Method != UsageBased {
		t.Errorf("methods: %s, %s", out[0].Method, out[1].Method)
	}
	wantKWh := 500 * Current.EnergyPerUnit["vcpu_hours"] * Current.PUE
	if math.Abs(out[1].EnergyKWh-wantKWh) > 1e-9 || math.Abs(out[1].CO2eKg-wantKWh*400/1000) > 1e-9 {
		t.Errorf("usage-based energy/carbon wrong: %+v", out[1])
	}
	if skipped[SkipNoRegion] != 1 || skipped[SkipNoGridData] != 1 || skipped[SkipCurrency] != 1 {
		t.Errorf("skips = %v", skipped)
	}
	if out[0].FunctionalUnits != nil || out[0].SCIScore != nil {
		t.Error("SCI must be NULL when R is unknown (never guessed)")
	}
	if !out[0].PeriodEnd.Equal(d1.Add(24*time.Hour)) || out[0].MethodologyVersion != MethodologyVersion {
		t.Errorf("period/version: %+v", out[0])
	}
}

func TestComputeSCIRowsSumToDaySCI(t *testing.T) {
	rows := []UsageDay{
		{Provider: "aws", ServiceName: "EC2", ServiceCategory: "compute", RegionID: "r", Currency: "USD", BilledCost: 100, Day: d1},
		{Provider: "aws", ServiceName: "S3", ServiceCategory: "storage", RegionID: "r", Currency: "USD", BilledCost: 40, Day: d1},
	}
	const r = 10_000.0
	out, _, err := Compute(context.Background(), rows, fixedIntensity(map[string]float64{"r": 300}), map[time.Time]float64{d1: r})
	if err != nil {
		t.Fatal(err)
	}
	var sumSCI, grams float64
	for _, c := range out {
		if c.SCIScore == nil || c.FunctionalUnits == nil || *c.FunctionalUnits != r {
			t.Fatalf("SCI inputs missing: %+v", c)
		}
		sumSCI += *c.SCIScore
		grams += c.CO2eKg * 1000
	}
	want, _ := SCI(grams/300, 300, 0, r) // E*I/R for the whole day
	if math.Abs(sumSCI-want) > 1e-9 {
		t.Errorf("row shares sum to %v, day SCI is %v", sumSCI, want)
	}
}

func TestSCIFormula(t *testing.T) {
	got, err := SCI(10, 400, 1000, 100)
	if err != nil || got != 50 {
		t.Fatalf("SCI = %v, %v; want 50", got, err)
	}
	if _, err := SCI(1, 1, 1, 0); err == nil {
		t.Error("zero units accepted")
	}
}
