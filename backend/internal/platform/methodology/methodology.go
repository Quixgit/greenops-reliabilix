// Package methodology holds the versioned carbon coefficient set. It is platform-level because several
// modules must agree on the active version (calculation, dashboards, reports, recommendations), while
// only the carbon domain interprets the coefficients. Versions are immutable data (ADR-0009).
package methodology

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Every *.json file here is one immutable coefficient set. A new set is a NEW file with a new version name; an
// existing file is never edited (a test pins the checksum of each). ActiveVersion picks the one used for new results.
//
//go:embed *.json
var setFiles embed.FS

// ActiveVersion is the coefficient set that new calculations, dashboards and reports use. Changing it is a release
// decision (the old sets stay available, so old results remain reproducible and comparable).
const ActiveVersion = "RLX-PROVISIONAL-1"

// Factors is a versioned coefficient set.
type Factors struct {
	Version string `json:"version"`
	Status  string `json:"status"`
	// Provenance says where the numbers come from, so a version label never implies more than is true.
	Provenance string `json:"provenance"`
	Notes      string `json:"notes"`
	// Caveats are shown to end users next to every figure derived from this set.
	Caveats            []string           `json:"caveats"`
	PUE                float64            `json:"pue"`
	EnergyPerUnit      map[string]float64 `json:"energy_kwh_per_unit"`
	CostBasedKWhPerUSD map[string]float64 `json:"cost_based_kwh_per_usd"`
	EmbodiedGPerKWh    float64            `json:"embodied_g_per_kwh"`
}

var (
	sets      = loadAll()
	checksums = map[string]string{}
	// Current is the active coefficient set; Version is stored with every calculation.
	Current = mustGet(ActiveVersion)
	Version = Current.Version
)

func loadAll() map[string]Factors {
	entries, err := setFiles.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("methodology: %v", err))
	}
	out := map[string]Factors{}
	for _, e := range entries {
		b, err := setFiles.ReadFile(e.Name())
		if err != nil {
			panic(fmt.Sprintf("methodology: %v", err))
		}
		f := mustLoad(b)
		if _, dup := out[f.Version]; dup {
			panic("methodology: duplicate version " + f.Version)
		}
		out[f.Version] = f
		sum := sha256.Sum256(b)
		checksums[f.Version] = hex.EncodeToString(sum[:])
	}
	return out
}

func mustGet(v string) Factors {
	f, ok := sets[v]
	if !ok {
		panic("methodology: active version not found: " + v)
	}
	return f
}

// Get returns a coefficient set by version.
func Get(version string) (Factors, bool) {
	f, ok := sets[version]
	return f, ok
}

// Versions lists every known set, oldest name first.
func Versions() []Factors {
	out := make([]Factors, 0, len(sets))
	for _, f := range sets {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out
}

// Checksum is the SHA-256 of a set's file, for the immutability test.
func Checksum(version string) string { return checksums[version] }

func mustLoad(b []byte) Factors {
	var f Factors
	if err := json.Unmarshal(b, &f); err != nil {
		panic(fmt.Sprintf("methodology: bad factors file: %v", err))
	}
	if f.Provenance == "" || f.Status == "provisional" && len(f.Caveats) == 0 {
		panic("methodology: provenance and (for provisional sets) caveats are required")
	}
	if f.Version == "" || f.PUE < 1 || len(f.EnergyPerUnit) == 0 || len(f.CostBasedKWhPerUSD) == 0 {
		panic("methodology: incomplete factors file")
	}
	return f
}

// EmbodiedIncluded reports whether the set accounts for embodied emissions (the M term of SCI).
func (f Factors) EmbodiedIncluded() bool { return f.EmbodiedGPerKWh > 0 }
