package geo

import "testing"

func TestRoundTripEstimates(t *testing.T) {
	cases := []struct {
		a, b     string
		min, max int
	}{
		{"canadacentral", "canadaeast", 6, 16},      // Toronto - Quebec: the same country, next door
		{"canadacentral", "francecentral", 70, 120}, // across the Atlantic
		{"us-east-1", "us-west-2", 45, 80},          // coast to coast
		{"eu-west-1", "eu-central-1", 15, 35},
	}
	for _, c := range cases {
		got, ok := RTTms(c.a, c.b)
		if !ok || got < c.min || got > c.max {
			t.Errorf("%s -> %s = %d ms (known %v), want %d-%d", c.a, c.b, got, ok, c.min, c.max)
		}
	}
	if _, ok := RTTms("us-east-1", "nowhere"); ok {
		t.Error("an unknown region must not be placed")
	}
	if d, _ := DistanceKm("us-east-1", "us-east-1"); d != 0 {
		t.Errorf("same place, distance %v", d)
	}
}

func TestLatencyIsMeasuredWhereAProviderPublishesIt(t *testing.T) {
	az, ok := Between("canadacentral", "canadaeast")
	if !ok || az.Basis != BasisMeasuredAzure || !az.Measured() || az.Ms < 5 || az.Ms > 25 || az.ErrorPct != 0 {
		t.Errorf("two Azure regions are a published measurement: %+v", az)
	}
	aw, ok := Between("us-east-1", "eu-west-1")
	if !ok || aw.Basis != BasisMeasuredAWS || aw.Ms < 50 || aw.Ms > 110 {
		t.Errorf("two AWS regions are a CloudPing measurement: %+v", aw)
	}
	gcp, ok := Between("us-central1", "europe-west1")
	if !ok || gcp.Basis != BasisEstimated || gcp.Measured() || gcp.ErrorPct == 0 {
		t.Errorf("GCP publishes no table: its pairs are estimates that say how far off they may be: %+v", gcp)
	}
	cross, _ := Between("us-east-1", "eastus")
	if cross.Basis != BasisEstimated {
		t.Errorf("a pair across clouds has no measurement: %+v", cross)
	}
	if same, ok := Between("eastus", "eastus"); !ok || same.Basis != BasisEstimated || same.Ms > 8 {
		t.Errorf("a region to itself is a short hop: %+v", same)
	}
}

func TestEstimateIsCloseToTheMeasuredPairsItWasFittedOn(t *testing.T) {
	// The fitted line must not drift from the data it came from: across the measured Azure pairs, half are within 25% of it.
	within, total := 0, 0
	for a, row := range table.Azure.Ms {
		for b, ms := range row {
			km, ok := DistanceKm(a, b)
			if !ok || a == b || ms < 4 {
				continue
			}
			est := table.Fit.Intercept + table.Fit.PerKm*km
			total++
			if d := est - float64(ms); d < 0.25*float64(ms) && -d < 0.25*float64(ms) {
				within++
			}
		}
	}
	if total < 500 || float64(within)/float64(total) < 0.5 {
		t.Errorf("the estimate is within 25%% of the measurement for %d of %d pairs", within, total)
	}
}
