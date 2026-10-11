package geo

import (
	_ "embed"
	"encoding/json"
	"math"
)

// How a latency figure was obtained. The console shows it next to every figure.
const (
	BasisMeasuredAzure = "measured_azure" // published by Microsoft: measured between two Azure regions
	BasisMeasuredAWS   = "measured_aws"   // CloudPing: TCP round trip between two AWS regions, median over a year
	BasisEstimated     = "estimated"      // from distance, with a line fitted to the measured pairs
)

// Latency is the round-trip time between two regions and where the figure comes from.
type Latency struct {
	Ms         int
	DistanceKm float64 // 0 when a region is not on the map
	Basis      string
	// ErrorPct is how far an estimate is typically off (80% of the measured pairs are within it); 0 for a measured figure.
	ErrorPct int
}

// Measured reports whether the figure is a measurement rather than an estimate.
func (l Latency) Measured() bool { return l.Basis != BasisEstimated }

//go:embed latency.json
var latencyJSON []byte

var table struct {
	Azure struct {
		Ms        map[string]map[string]int `json:"ms"`
		Published string                    `json:"published"`
	} `json:"azure"`
	AWS struct {
		Ms      map[string]map[string]int `json:"ms"`
		Fetched string                    `json:"fetched"`
	} `json:"aws"`
	Fit struct {
		Intercept float64 `json:"intercept_ms"`
		PerKm     float64 `json:"per_km_ms"`
		P80       int     `json:"p80_error_pct"`
	} `json:"fit"`
}

func init() {
	if err := json.Unmarshal(latencyJSON, &table); err != nil {
		panic("geo: latency.json is not valid: " + err.Error())
	}
}

// Between is the round trip between two regions: a measured figure when the provider (or CloudPing for AWS) published one for
// this pair, otherwise an estimate from the distance. The estimate is a line fitted to thousands of measured pairs, not a guess
// of "light in glass"; ErrorPct says how far it is typically off. It is false when a pair is unknown and not on the map.
//
// Latency is not symmetric in measurements (two directions are two measurements): the larger of the two published is used, the
// honest choice for a round trip that has to work both ways.
func Between(a, b string) (Latency, bool) {
	km, onMap := DistanceKm(a, b)
	pick := func(m map[string]map[string]int) (int, bool) {
		x, ok1 := m[a][b]
		y, ok2 := m[b][a]
		switch {
		case ok1 && ok2:
			return max(x, y), true
		case ok1:
			return x, true
		case ok2:
			return y, true
		}
		return 0, false
	}
	if ms, ok := pick(table.Azure.Ms); ok && a != b {
		return Latency{Ms: max(1, ms), DistanceKm: km, Basis: BasisMeasuredAzure}, true
	}
	if ms, ok := pick(table.AWS.Ms); ok && a != b {
		return Latency{Ms: max(1, ms), DistanceKm: km, Basis: BasisMeasuredAWS}, true
	}
	if !onMap {
		return Latency{}, false
	}
	return Latency{Ms: int(math.Max(1, math.Round(table.Fit.Intercept+table.Fit.PerKm*km))), DistanceKm: km, Basis: BasisEstimated, ErrorPct: table.Fit.P80}, true
}
