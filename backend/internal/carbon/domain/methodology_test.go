package domain

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
)

var methodologyGet = methodology.Get

func TestFactorsLoaded(t *testing.T) {
	if MethodologyVersion != "RLX-CCF-1" || Current.Status != "provisional" || Current.PUE < 1 {
		t.Fatalf("factors: %+v", Current)
	}
}

// Guards the units of every coefficient: a figure that is off by a factor of 1000 (per TB instead of per GB) is
// not caught by the arithmetic tests above, but is obvious against what the hardware can plausibly draw.
func TestCoefficientsArePlausible(t *testing.T) {
	cases := []struct {
		kind     UsageKind
		amount   float64
		min, max float64 // plausible kWh range for this amount, including PUE
	}{
		{KindVCPUHours, 1, 0.0005, 0.01},    // one vCPU-hour: roughly 1-5 W
		{KindMemoryGBHrs, 1, 0.0001, 0.002}, // one GB-hour of RAM: a fraction of a watt
		{KindStorageGBMo, 1000, 0.1, 2},     // one TB-month of stored data: about 0.3-0.6 kWh
	}
	for _, c := range cases {
		kwh, err := EnergyKWh(c.kind, c.amount)
		if err != nil || kwh < c.min || kwh > c.max {
			t.Errorf("%s x %v = %v kWh, outside the plausible range [%v, %v] (err %v)", c.kind, c.amount, kwh, c.min, c.max, err)
		}
	}
}

// A set says exactly where its numbers come from. The active set cites the published constants it was derived from and
// still names what it leaves out; the earlier author-estimate set must never be mistaken for a CCF import.
func TestMethodologyIsHonestAboutItsOrigin(t *testing.T) {
	if Current.Provenance == "" || len(Current.Caveats) == 0 {
		t.Fatalf("a set must say where it comes from and what it leaves out: %+v", Current)
	}
	if Current.EmbodiedIncluded() {
		t.Error("this set does not model embodied emissions and must not claim to")
	}
	joined := strings.Join(Current.Caveats, " ")
	if !strings.Contains(joined, "Embodied") || !strings.Contains(joined, "not sourced") {
		t.Errorf("caveats must name the missing embodied term and the unsourced cost-based factors: %v", Current.Caveats)
	}
	if !strings.Contains(Current.Notes, "AwsFootprintEstimationConstants") || !strings.Contains(Current.Notes, "0.74") {
		t.Errorf("a set derived from published constants must cite the file and the numbers it used: %q", Current.Notes)
	}
	old, ok := methodologyGet("RLX-PROVISIONAL-1")
	if !ok || !strings.Contains(old.Provenance, "unverified") || strings.Contains(strings.ToUpper(old.Version), "CCF") {
		t.Errorf("the author-estimate set must stay labelled as unverified: %+v", old)
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
