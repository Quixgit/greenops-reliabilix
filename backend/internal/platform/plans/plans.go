// Package plans is the plan catalog (ADR-0033, implementing ADR-0022): a plan is a named preset of the limit and module
// settings of the settings engine. Assigning a plan writes those values for one organization as locked operator
// values; nothing here enforces anything by itself, the existing gates and limits do.
package plans

import (
	"encoding/json"
	"sort"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// Plan is one entry of the catalog. Limits use 0 for "unlimited" (the platform maximum applies where the setting says so).
type Plan struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// PriceMonthlyUSD is the list price shown to customers: 0 is free, nil is "individual terms". It is what the plan page
	// shows while a payment provider has no amount of its own; the provider's amount is what is charged (ADR-0038).
	PriceMonthlyUSD *int            `json:"price_monthly_usd"`
	Limits          map[string]int  `json:"limits"`  // settings key -> value
	Modules         map[string]bool `json:"modules"` // settings key -> enabled
}

func usd(n int) *int { return &n }

func modules(automation, kubernetes bool) map[string]bool {
	return map[string]bool{
		settings.KeyModuleAutomation: automation, settings.KeyModuleKubernetes: kubernetes, settings.KeyModuleReports: true,
		settings.KeyModuleRecommendations: true, settings.KeyModuleNotifications: true, settings.KeyModuleCIGate: true,
	}
}

func limits(projects, connections, members, destinations, trackedSpend int) map[string]int {
	return map[string]int{
		settings.KeyLimitProjects: projects, settings.KeyLimitConnections: connections,
		settings.KeyLimitMembers: members, settings.KeyLimitDestinations: destinations, settings.KeyLimitTrackedSpend: trackedSpend,
	}
}

// Catalog returns the plans in display order. The numbers are a proposal to confirm before the first paying customer; the
// reasoning, the market comparison and the list prices to set at the payment providers are in docs/product/07-pricing.md.
// What costs the platform money is the number of connected cloud accounts (each one is synced, recalculated and drives grid
// data requests), so that is the limit that separates the plans; seats are cheap and stay generous but bounded.
func Catalog() []Plan {
	return []Plan{
		{ID: "free", Name: "Free", Description: "Try the platform with one cloud account.", PriceMonthlyUSD: usd(0), Limits: limits(3, 1, 3, 2, 2500), Modules: modules(false, false)},
		{ID: "team", Name: "Team", Description: "A product team: several accounts, Kubernetes and automation plans.", PriceMonthlyUSD: usd(45), Limits: limits(10, 5, 10, 10, 10000), Modules: modules(true, true)},
		{ID: "business", Name: "Business", Description: "Many teams and accounts.", PriceMonthlyUSD: usd(225), Limits: limits(50, 25, 50, 30, 40000), Modules: modules(true, true)},
		{ID: "enterprise", Name: "Enterprise", Description: "No platform quotas; individual terms are set per organization.", Limits: limits(0, 0, 0, 0, 0), Modules: modules(true, true)},
	}
}

// Lookup finds a plan by id.
func Lookup(id string) (Plan, bool) {
	for _, p := range Catalog() {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// Values returns the settings values the plan writes for an organization, keys sorted for stable history.
func (p Plan) Values() map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	keys := make([]string, 0, len(p.Limits)+len(p.Modules))
	for k := range p.Limits {
		keys = append(keys, k)
	}
	for k := range p.Modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var v any
		if n, ok := p.Limits[k]; ok {
			v = n
		} else {
			v = p.Modules[k]
		}
		b, _ := json.Marshal(v)
		out[k] = b
	}
	return out
}
