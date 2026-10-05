// Package methodology holds the versioned carbon coefficient set. It is platform-level because several
// modules must agree on the active version (calculation, dashboards, reports, recommendations), while
// only the carbon domain interprets the coefficients. Versions are immutable data (ADR-0009).
package methodology

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed ccf-2026.1.json
var factorsJSON []byte

// Factors is a versioned coefficient set.
type Factors struct {
	Version            string             `json:"version"`
	Status             string             `json:"status"`
	Notes              string             `json:"notes"`
	PUE                float64            `json:"pue"`
	EnergyPerUnit      map[string]float64 `json:"energy_kwh_per_unit"`
	CostBasedKWhPerUSD map[string]float64 `json:"cost_based_kwh_per_usd"`
	EmbodiedGPerKWh    float64            `json:"embodied_g_per_kwh"`
}

// Current is the active coefficient set; Version is stored with every calculation.
var (
	Current = mustLoad(factorsJSON)
	Version = Current.Version
)

func mustLoad(b []byte) Factors {
	var f Factors
	if err := json.Unmarshal(b, &f); err != nil {
		panic(fmt.Sprintf("methodology: bad factors file: %v", err))
	}
	if f.Version == "" || f.PUE < 1 || len(f.EnergyPerUnit) == 0 || len(f.CostBasedKWhPerUSD) == 0 {
		panic("methodology: incomplete factors file")
	}
	return f
}
