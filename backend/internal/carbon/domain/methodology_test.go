package domain

import (
	"errors"
	"math"
	"testing"
)

func TestFactorsLoaded(t *testing.T) {
	if MethodologyVersion != "CCF-2026.1" || Current.Status != "provisional" || Current.PUE < 1 {
		t.Fatalf("factors: %+v", Current)
	}
}

func TestCalculate(t *testing.T) {
	r, err := Calculate(Input{Kind: KindVCPUHours, Amount: 1000, GridIntensityGPerKWh: 400})
	if err != nil {
		t.Fatal(err)
	}
	wantKWh := 1000 * Current.EnergyPerUnit["vcpu_hours"] * Current.PUE
	if math.Abs(r.EnergyKWh-wantKWh) > 1e-9 || math.Abs(r.CO2eKg-wantKWh*0.4) > 1e-9 || r.MethodologyVersion != MethodologyVersion {
		t.Errorf("result %+v", r)
	}
}

func TestCalculateRejectsBadInput(t *testing.T) {
	for _, in := range []Input{
		{Kind: "nope", Amount: 1},
		{Kind: KindVCPUHours, Amount: -1},
		{Kind: KindVCPUHours, Amount: 1, GridIntensityGPerKWh: -5},
	} {
		if _, err := Calculate(in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: err = %v", in, err)
		}
	}
}

func TestCostBasedNeverNegative(t *testing.T) {
	if kwh, ok := CostBasedEnergyKWh("compute", -50); !ok || kwh != 0 {
		t.Errorf("credit produced energy %v", kwh)
	}
	if kwh, _ := CostBasedEnergyKWh("compute", 10); kwh <= 0 {
		t.Error("positive spend must produce energy")
	}
	if a, _ := CostBasedEnergyKWh("unknown-category", 10); a <= 0 {
		t.Error("unknown category must fall back to other")
	}
}

func TestKindForUnit(t *testing.T) {
	for u, want := range map[string]UsageKind{"vCPU-Hrs": KindVCPUHours, "GB-Hours": KindMemoryGBHrs, "GB-Mo": KindStorageGBMo} {
		if got, ok := KindForUnit(u); !ok || got != want {
			t.Errorf("%s -> %v %v", u, got, ok)
		}
	}
	if _, ok := KindForUnit("Requests"); ok {
		t.Error("unknown unit accepted")
	}
}
