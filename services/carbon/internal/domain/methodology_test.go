package domain

import (
	"errors"
	"math"
	"testing"
)

func TestCalculate(t *testing.T) {
	r, err := Calculate(Input{Kind: KindVCPUHours, Amount: 1000, GridIntensityGPerKWh: 400})
	if err != nil {
		t.Fatal(err)
	}
	wantKWh := 1000 * 0.0021 * 1.135
	if math.Abs(r.EnergyKWh-wantKWh) > 1e-9 {
		t.Errorf("kwh = %v, want %v", r.EnergyKWh, wantKWh)
	}
	if math.Abs(r.CO2eKg-wantKWh*0.4) > 1e-9 {
		t.Errorf("co2e = %v", r.CO2eKg)
	}
	if r.MethodologyVersion != MethodologyVersion {
		t.Errorf("methodology version missing")
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
