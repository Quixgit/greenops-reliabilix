package repository

import (
	"math"
	"testing"
)

func TestRelocationSavingNeverExceedsTheFootprint(t *testing.T) {
	// Toronto: 46 kg over the period, 143 g/kWh now; Oslo 25 g/kWh now. The grid is 83% cleaner, so about 83% of 46 kg.
	got := relocationSavingKg(46, 143, 25)
	if math.Abs(got-38.0) > 0.2 {
		t.Errorf("saving = %.2f kg, want about 38", got)
	}
	if got > 46 {
		t.Errorf("a saving of %.1f kg is more than the region emitted", got)
	}
	for _, c := range [][3]float64{{0, 143, 25}, {46, 0, 25}, {46, 100, 100}, {46, 100, 120}} {
		if relocationSavingKg(c[0], c[1], c[2]) != 0 {
			t.Errorf("no saving expected for %v", c)
		}
	}
}
