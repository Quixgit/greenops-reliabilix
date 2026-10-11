package domain

import (
	"encoding/json"
	"fmt"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/geo"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Workload is the observed carbon and cost of one service in one region over a window.
type Workload struct {
	Provider, Service, Region string
	EnergyKWh, CarbonKg, Cost float64
}

// RegionShiftInput is everything the generator needs; it performs no I/O.
type RegionShiftInput struct {
	Workloads      []Workload
	WindowDays     int                // length of the observed window
	DataDays       int                // days of the window that actually have data
	Intensity      map[string]float64 // latest gCO2e/kWh per region
	AllowedRegions []string           // project data-residency allow-list
	PriceIndex     map[string]float64 // optional relative price per region of virtual machine compute (the clouds' regions never collide)
	// ServiceIndex is the same for managed services, keyed by family ("azure:postgresql", "azure:redis"); optional.
	ServiceIndex    map[string]map[string]float64
	MinReductionPct float64 // default 10
	MinMonthlyKg    float64 // default 1
	// MaxLatencyMs is the most round-trip time a move may add (an estimate from the distance between the regions). A region
	// beyond it is shown among the options but never proposed. 0 means DefaultMaxLatencyMs.
	MaxLatencyMs int
	Now          time.Time
}

// DefaultMaxLatencyMs lets a workload move within its country or neighbouring ones, not across an ocean.
const DefaultMaxLatencyMs = 60

// RegionOption is one place a workload could run instead, with what it would change on every axis that matters, so a person
// can weigh them: it is rarely the cleanest region that is also the cheapest and the closest.
type RegionOption struct {
	Region       string   `json:"region"`
	GPerKWh      float64  `json:"g_per_kwh"`
	ReductionPct float64  `json:"carbon_reduction_pct"`
	ReductionKg  float64  `json:"carbon_reduction_kg_month"`
	LatencyMs    *int     `json:"latency_ms,omitempty"` // estimated round trip from where it runs now; nil when a region is not on the map
	DistanceKm   *int     `json:"distance_km,omitempty"`
	LatencyBasis string   `json:"latency_basis,omitempty"`     // measured_azure, measured_aws or estimated (geo.Basis*)
	LatencyError int      `json:"latency_error_pct,omitempty"` // how far an estimate is typically off; 0 for a measurement
	CostChange   *float64 `json:"cost_change_pct,omitempty"`   // from the price index, EC2 compute only; nil when not known
	WithinBudget bool     `json:"within_latency_budget"`
	Chosen       bool     `json:"chosen,omitempty"`
	Tags         []string `json:"tags,omitempty"` // lowest_carbon, closest, cheapest
}

var (
	awsRegionRe = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)
	gcpRegionRe = regexp.MustCompile(`^[a-z]+(-[a-z]+)?\d+$`)
	azRegionRe  = regexp.MustCompile(`^[a-z]+\d*$`)
)

// RegionCloud tells which cloud a region id belongs to from its shape: "eu-central-1" is aws, "europe-west6" is gcp,
// "westeurope" is azure; "" when it is none of them. A workload is only ever moved within its own cloud.
func RegionCloud(id string) string {
	switch {
	case awsRegionRe.MatchString(id):
		return "aws"
	case strings.Contains(id, "-") && gcpRegionRe.MatchString(id):
		return "gcp"
	case azRegionRe.MatchString(id):
		return "azure"
	}
	return ""
}

// GenerateRegionShift proposes moving workloads to a cleaner region.
//
// Hard rules:
//   - It never proposes a region outside the project's allow-list; with an empty list (residency unknown) it
//     proposes nothing at all.
//   - Cost impact is estimated only when a price index exists for both regions; otherwise it is reported
//     as not estimated rather than guessed.
//   - The carbon effect is computed from stored grid intensity, scaled to a month.
func GenerateRegionShift(in RegionShiftInput) []Recommendation {
	if len(in.AllowedRegions) == 0 || in.WindowDays <= 0 {
		return nil
	}
	budget := in.MaxLatencyMs
	if budget <= 0 {
		budget = DefaultMaxLatencyMs
	}
	minPct, minKg := in.MinReductionPct, in.MinMonthlyKg
	if minPct == 0 {
		minPct = 10
	}
	if minKg == 0 {
		minKg = 1
	}
	monthly := 30.0 / float64(in.WindowDays)
	dataFactor := math.Min(1, float64(in.DataDays)/30)

	var out []Recommendation
	for _, w := range in.Workloads {
		cur, ok := in.Intensity[w.Region]
		if !ok || cur <= 0 || w.CarbonKg <= 0 {
			continue
		}
		opts := regionOptions(in, w, cur, monthly, budget)
		var pick *RegionOption
		for i := range opts {
			o := &opts[i]
			if !o.WithinBudget || o.ReductionPct < minPct || o.ReductionKg < minKg {
				continue
			}
			if pick == nil || o.ReductionPct > pick.ReductionPct || (o.ReductionPct == pick.ReductionPct && latencyOf(o) < latencyOf(pick)) {
				pick = o
			}
		}
		if pick == nil {
			continue
		}
		pick.Chosen = true
		target, pct, kg := pick.Region, pick.ReductionPct, pick.ReductionKg
		tagOptions(opts)
		rec := Recommendation{
			Type: RegionShift, Title: fmt.Sprintf("Move %s from %s to %s", w.Service, w.Region, target),
			Provider: w.Provider, ServiceName: w.Service, CurrentRegion: w.Region, RecommendedRegion: target,
			CarbonReductionPct: round(pct, 1), CarbonReductionKg: round(kg, 1), CostBasis: "not_estimated", Status: Open,
			Confidence: round(math.Min(0.9, (0.5+0.4*math.Min(1, pct/50))*dataFactor), 2),
			Compliance: ComplianceCheck{Residency: "passed", AllowedRegions: append([]string(nil), in.AllowedRegions...), CheckedAt: in.Now},
		}
		// An index describes one kind of service (virtual machine compute, or a managed database or cache): it is applied to that
		// service only, never to storage, networking or functions.
		if idx := priceIndexFor(in, w); idx != nil && idx[w.Region] > 0 && idx[target] > 0 {
			a, b := idx[w.Region], idx[target]
			impact := round(w.Cost*monthly*(b/a-1), 2)
			rec.CostImpact, rec.CostBasis = &impact, "region_price_index"
		}
		rec.Details = map[string]string{"latency_budget_ms": strconv.Itoa(budget)}
		if pick.LatencyMs != nil {
			rec.Details["latency_ms"], rec.Details["distance_km"] = strconv.Itoa(*pick.LatencyMs), strconv.Itoa(*pick.DistanceKm)
			rec.Details["latency_basis"] = pick.LatencyBasis
		}
		if b, err := json.Marshal(trimOptions(opts, 6)); err == nil {
			rec.Details["options"] = string(b)
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CarbonReductionKg > out[j].CarbonReductionKg })
	return out
}

func latencyOf(o *RegionOption) int {
	if o.LatencyMs == nil {
		return 1 << 20 // unknown is the least sure
	}
	return *o.LatencyMs
}

// regionOptions lists every allowed region of the workload's cloud that is cleaner than where it runs, with what moving there
// would change: carbon, round-trip time and, where the price index knows both regions, cost.
func regionOptions(in RegionShiftInput, w Workload, cur, monthly float64, budget int) []RegionOption {
	var opts []RegionOption
	for _, r := range in.AllowedRegions {
		if r == w.Region {
			continue
		}
		if c := RegionCloud(r); c != "" && w.Provider != "" && c != strings.ToLower(w.Provider) {
			continue // another cloud's region is not a move, it is a migration
		}
		g, ok := in.Intensity[r]
		if !ok || g >= cur {
			continue
		}
		pct := (cur - g) / cur * 100
		o := RegionOption{Region: r, GPerKWh: round(g, 0), ReductionPct: round(pct, 1), ReductionKg: round(w.CarbonKg*monthly*pct/100, 1), WithinBudget: true}
		if l, ok := geo.Between(w.Region, r); ok {
			d, ms := int(math.Round(l.DistanceKm)), l.Ms
			o.LatencyMs, o.DistanceKm, o.WithinBudget, o.LatencyBasis, o.LatencyError = &ms, &d, ms <= budget, l.Basis, l.ErrorPct
		} // a region the map does not know cannot be ruled out by distance: it stays within the budget, and the card says the latency is not estimated
		if idx := priceIndexFor(in, w); idx != nil && idx[w.Region] > 0 && idx[r] > 0 {
			a, b := idx[w.Region], idx[r]
			c := round((b/a-1)*100, 1)
			o.CostChange = &c
		}
		opts = append(opts, o)
	}
	sort.Slice(opts, func(i, j int) bool { return opts[i].ReductionPct > opts[j].ReductionPct })
	return opts
}

// tagOptions marks the best option on each axis among those a person could really pick (within the latency budget).
func tagOptions(opts []RegionOption) {
	low, near, cheap := -1, -1, -1
	for i := range opts {
		o := &opts[i]
		if !o.WithinBudget {
			continue
		}
		if low < 0 || o.ReductionPct > opts[low].ReductionPct {
			low = i
		}
		if o.LatencyMs != nil && (near < 0 || *o.LatencyMs < *opts[near].LatencyMs) {
			near = i
		}
		if o.CostChange != nil && (cheap < 0 || *o.CostChange < *opts[cheap].CostChange) {
			cheap = i
		}
	}
	if low >= 0 {
		opts[low].Tags = append(opts[low].Tags, "lowest_carbon")
	}
	if near >= 0 {
		opts[near].Tags = append(opts[near].Tags, "closest")
	}
	if cheap >= 0 {
		opts[cheap].Tags = append(opts[cheap].Tags, "cheapest")
	}
}

// trimOptions keeps the chosen option and the best of the rest, at most n.
func trimOptions(opts []RegionOption, n int) []RegionOption {
	if len(opts) <= n {
		return opts
	}
	out := append([]RegionOption(nil), opts[:n]...)
	for i := n; i < len(opts); i++ {
		if opts[i].Chosen {
			out[n-1] = opts[i]
		}
	}
	return out
}

// isVMCompute reports whether a billed service is virtual machine compute: AWS EC2, Azure Virtual Machines, Google Compute Engine.
func isVMCompute(provider, service string) bool {
	s := strings.ToLower(strings.TrimSpace(service))
	switch strings.ToLower(provider) {
	case "aws":
		return strings.Contains(s, "elastic compute cloud") || s == "amazon ec2" || s == "ec2"
	case "azure":
		return s == "virtual machines"
	case "gcp":
		return s == "compute engine"
	}
	return false
}

// ManagedFamily is the price-index family of a managed service ("" when no index describes it).
func ManagedFamily(provider, service string) string {
	s := strings.ToLower(strings.TrimSpace(service))
	switch strings.ToLower(provider) {
	case "azure":
		switch {
		case s == "azure database for postgresql":
			return "azure:postgresql"
		case s == "redis cache" || s == "azure cache for redis":
			return "azure:redis"
		}
	case "aws":
		switch {
		case s == "amazon relational database service" || s == "amazon rds":
			return "aws:rds"
		case s == "amazon elasticache" || s == "amazon elasticache service":
			return "aws:elasticache"
		}
	case "gcp":
		switch {
		case s == "cloud sql":
			return "gcp:cloudsql"
		case s == "cloud memorystore for redis" || s == "memorystore for redis" || s == "memorystore":
			return "gcp:memorystore"
		}
	}
	return ""
}

// priceIndexFor returns the region index that describes this workload's service, or nil when none does.
func priceIndexFor(in RegionShiftInput, w Workload) map[string]float64 {
	if isVMCompute(w.Provider, w.Service) {
		return in.PriceIndex
	}
	if fam := ManagedFamily(w.Provider, w.Service); fam != "" {
		return in.ServiceIndex[fam]
	}
	return nil
}

func round(v float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(v*p) / p
}
