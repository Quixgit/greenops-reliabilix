package application

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// gridSource is what the console knows about a grid-intensity source beyond its data: the kind of figure and its licence.
// A source exists only when there is a connector for it, so this is a catalog in code (ADR-0032).
type gridSource struct {
	id, label, granularity, licence string
	computed                        bool
}

var gridSources = []gridSource{
	{"electricitymaps", "Electricity Maps", "hourly", "Commercial API; zones and history depend on the plan", false},
	{"uk_carbon_intensity", "UK Carbon Intensity", "hourly", "Open data, CC BY 4.0", false},
	{"eia", "EIA (US generation mix)", "hourly", "U.S. government data, public domain", true},
	{"energy_charts", "Energy-Charts (Fraunhofer ISE)", "hourly", "CC BY 4.0: credit Fraunhofer ISE", true},
	{"entsoe", "ENTSO-E Transparency (Europe, generation mix)", "hourly", "Free with a token; ENTSO-E re-use terms: credit ENTSO-E", true},
	{"ieso", "IESO (Ontario, generation mix)", "hourly", "Open data from the Independent Electricity System Operator, no key", true},
	{"hydroquebec", "Hydro-Québec (Quebec, generation mix)", "hourly", "Open data from Hydro-Québec (CC BY 4.0): credit Hydro-Québec", true},
	{"ons", "ONS (Brazil, generation mix)", "hourly", "Open data of the Operador Nacional do Sistema Elétrico (CC BY), no key", true},
	{"ember", "Ember", "monthly", "CC BY 4.0: credit Ember", false},
}

var aggregationRules = []domain.AggregationRule{
	{Code: "one_source", Title: "One source per region and day", Text: "A day of usage is calculated with the intensity of a single source, the first in the order that has enough readings. Sources are never averaged together: mixing them would hide which licence and which method a figure rests on."},
	{Code: "order", Title: "Order of preference", Text: "1. a source with enough hourly readings that day; 2. a source's monthly published figure; 3. the first source with some readings that day (marked partial); 4. a source's last reading before the day, within its accepted age; 5. the latest reading that exists, marked as an estimate."},
	{Code: "measures", Title: "Average and marginal are different quantities", Text: "Only the average intensity of all generation feeds carbon figures. Marginal intensity (the generator that would change at the margin) is stored separately when a source provides it and is never mixed into an average."},
	{Code: "units", Title: "Units and time", Text: "Intensity is grams of CO2e per kWh. Readings are hourly and in UTC; a day is a UTC day. A source that publishes another interval is brought to hourly readings or kept as a monthly figure, never stretched silently."},
	{Code: "currency", Title: "Money", Text: "Costs are brought to the report currency at the ECB reference rate of the day. A sum in a currency without a rate is left out of the total, and the figure says which currency and how many sums."},
	{Code: "provenance", Title: "Every figure keeps its origin", Text: "A calculation row records the source and the basis of its intensity. Rows calculated before this was recorded stay without an origin: it is not guessed afterwards."},
	{Code: "recalculation", Title: "Changes apply forward", Text: "A change of policy or of methodology applies to the next calculation. History is recalculated only on request; results of other methodology versions are kept, so versions can be compared."},
}

// policyFromSettings reads the source policy in force from the platform settings; a missing value is the catalog default.
func (s Service) policyFromSettings(ctx context.Context) domain.AggregationPolicy {
	p := domain.AggregationPolicy{
		Priority: []string{"electricitymaps", "uk_carbon_intensity", "eia", "energy_charts", "entsoe", "ieso", "hydroquebec", "ons", "ember"}, Overrides: map[string]string{},
		MinDayReadings: 6, MaxAgeHours: 72, SourceMaxAgeHours: map[string]int{},
	}
	if s.Settings == nil {
		return p
	}
	items, err := s.Settings.PlatformItems(ctx)
	if err != nil {
		return p
	}
	for _, it := range items {
		if it.Def == nil {
			continue
		}
		var list []string
		var n int64
		switch it.Key {
		case settings.KeyCarbonPriority:
			if json.Unmarshal(it.Value, &list) == nil && len(list) > 0 {
				p.Priority = list
			}
		case settings.KeyCarbonOverrides:
			if json.Unmarshal(it.Value, &list) == nil {
				for _, e := range list {
					if region, src, ok := strings.Cut(e, "="); ok {
						p.Overrides[region] = src
					}
				}
			}
		case settings.KeyCarbonSourceAge:
			if json.Unmarshal(it.Value, &list) == nil {
				for _, e := range list {
					if src, h, ok := strings.Cut(e, "="); ok {
						if v, err := strconv.Atoi(h); err == nil && v > 0 {
							p.SourceMaxAgeHours[src] = v
						}
					}
				}
			}
		case settings.KeyCarbonMinReadings:
			if json.Unmarshal(it.Value, &n) == nil && n > 0 {
				p.MinDayReadings = int(n)
			}
		case settings.KeyCarbonMaxAgeHours:
			if json.Unmarshal(it.Value, &n) == nil && n > 0 {
				p.MaxAgeHours = int(n)
			}
		case settings.KeyCarbonMaxRequests:
			if json.Unmarshal(it.Value, &n) == nil {
				p.DailyRequestCap = n
			}
		}
	}
	return p
}

// freshness grades the newest reading of a source: an hourly source is fresh for hours, a monthly one for weeks.
func freshness(granularity string, newest *time.Time, accepted time.Duration, now time.Time) string {
	if newest == nil {
		return "none"
	}
	age := now.Sub(*newest)
	fresh := 6 * time.Hour
	if granularity == "monthly" {
		fresh, accepted = 75*24*time.Hour, max(accepted, 120*24*time.Hour)
	}
	switch {
	case age <= fresh:
		return "fresh"
	case accepted <= 0 || age <= accepted:
		return "aging"
	default:
		return "stale"
	}
}

// Aggregation is the page "Aggregation & policies": the policy in force, each source with its place in it and the
// freshness of its data, and where the last days' figures took their intensity from.
func (s Service) Aggregation(ctx context.Context, now time.Time) (domain.Aggregation, error) {
	const provenanceDays = 30
	pol := s.policyFromSettings(ctx)
	rows, err := s.Repo.CarbonRegionSources(ctx)
	if err != nil {
		return domain.Aggregation{}, err
	}
	regions, err := s.Repo.CarbonRegions(ctx)
	if err != nil {
		return domain.Aggregation{}, err
	}
	prov, err := s.Repo.IntensityProvenance(ctx, provenanceDays)
	if err != nil {
		return domain.Aggregation{}, err
	}
	position := map[string]int{}
	for i, id := range pol.Priority {
		position[id] = i + 1
	}
	pinned := map[string]int{}
	for _, src := range pol.Overrides {
		pinned[src]++
	}
	out := domain.Aggregation{Policy: pol, Rules: aggregationRules, Regions: len(regions), Sources: []domain.AggregationSource{}}
	for _, g := range gridSources {
		a := domain.AggregationSource{ID: g.id, Label: g.label, Measure: "average", Granularity: g.granularity, Computed: g.computed, Licence: g.licence,
			Position: position[g.id], Pinned: pinned[g.id], MaxAgeHours: pol.MaxAgeHours}
		if h, ok := pol.SourceMaxAgeHours[g.id]; ok {
			a.MaxAgeHours = h
		}
		for _, r := range rows {
			if r.Provider != g.id {
				continue
			}
			a.Regions++
			a.Hourly30d += r.Hourly
			a.Months += r.Months
			if r.Newest != nil && (a.Newest == nil || r.Newest.After(*a.Newest)) {
				t := *r.Newest
				a.Newest = &t
			}
		}
		a.Freshness = freshness(g.granularity, a.Newest, time.Duration(a.MaxAgeHours)*time.Hour, now)
		out.Sources = append(out.Sources, a)
	}
	div, err := s.Repo.SourceDivergence(ctx, divergenceDays)
	if err != nil {
		return domain.Aggregation{}, err
	}
	out.Divergence = divergences(div, pol, regions, now)
	out.Provenance = domain.Provenance{Days: provenanceDays, Rows: prov}
	var attributed, precise int64
	for _, r := range prov {
		out.Provenance.Total += r.Rows
		if r.Source == "" {
			out.Provenance.Unattributed += r.Rows
			continue
		}
		attributed += r.Rows
		if r.Basis == "day_mean" {
			precise += r.Rows
		}
	}
	if attributed > 0 {
		out.Provenance.PreciseShare = float64(precise) / float64(attributed)
	}
	return out, nil
}

var regionIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{2,60}$`)

// ExplainIntensity says why a region and day got the intensity it did. The question is answered by the carbon engine
// through a port, so the console shows the decision the engine really takes.
func (s Service) ExplainIntensity(ctx context.Context, region, day string) (any, error) {
	if s.Explain == nil {
		return nil, domain.ErrNotFound
	}
	d, err := time.Parse(time.DateOnly, day)
	if err != nil || !regionIDRe.MatchString(region) || d.After(time.Now().Add(24*time.Hour)) || d.Before(time.Now().AddDate(-3, 0, 0)) {
		return nil, domain.ErrInvalid
	}
	return s.Explain(ctx, region, d)
}

// divergenceDays is the window of the comparison: long enough for a typical figure, short enough to follow a change.
const divergenceDays = 7

// divergences compares, per region, the typical hourly figure of every source that has one and holds it against the monthly
// national figure (Ember). Sources are never combined: this only shows where they disagree, so a person can see which figure a
// client would be shown and how far the others are from it. A region with a single hourly source and no anchor is left out, and a lone
// source is held against the national figure only once it has two days of readings: a few evening hours are not a typical value.
func divergences(rows []domain.DivergenceRow, pol domain.AggregationPolicy, inUse []string, now time.Time) []domain.Divergence {
	used := map[string]bool{}
	for _, r := range inUse {
		used[r] = true
	}
	label := map[string]string{}
	computed := map[string]bool{}
	for _, g := range gridSources {
		label[g.id], computed[g.id] = g.label, g.computed
	}
	position := map[string]int{}
	for i, id := range pol.Priority {
		position[id] = i + 1
	}
	byRegion := map[string][]domain.DivergenceRow{}
	var order []string
	for _, r := range rows {
		if _, ok := byRegion[r.Region]; !ok {
			order = append(order, r.Region)
		}
		byRegion[r.Region] = append(byRegion[r.Region], r)
	}
	var out []domain.Divergence
	for _, region := range order {
		d := domain.Divergence{Region: region, InUse: used[region], Sources: []domain.DivergenceSource{}}
		var hourly []domain.DivergenceSource
		for _, r := range byRegion[region] {
			if r.Kind == "annual" {
				if d.Inventory == nil || r.Newest.After(d.Inventory.Month) {
					d.Inventory = &domain.DivergenceAnchor{Source: r.Provider, Label: "Climate TRACE inventory ÷ Ember generation", Month: r.Newest, G: r.G}
				}
				continue
			}
			if position[r.Provider] == 0 {
				continue // not in the policy: never used, so not compared
			}
			if r.Kind == "monthly" {
				if d.Anchor == nil || r.Newest.After(d.Anchor.Month) {
					d.Anchor = &domain.DivergenceAnchor{Source: r.Provider, Label: label[r.Provider], Month: r.Newest, G: r.G}
				}
				continue
			}
			newest := r.Newest
			age := time.Duration(pol.MaxAgeHours) * time.Hour
			if h, ok := pol.SourceMaxAgeHours[r.Provider]; ok {
				age = time.Duration(h) * time.Hour
			}
			hourly = append(hourly, domain.DivergenceSource{ID: r.Provider, Label: label[r.Provider], G: r.G, Readings: r.N, Newest: &newest,
				Computed: computed[r.Provider], Fresh: age <= 0 || now.Sub(newest) <= age})
		}
		if len(hourly) == 0 || (len(hourly) < 2 && d.Anchor == nil && d.Inventory == nil) {
			continue
		}
		sort.Slice(hourly, func(i, j int) bool { return position[hourly[i].ID] < position[hourly[j].ID] })
		lo, hi := hourly[0].G, hourly[0].G
		for i := range hourly {
			lo, hi = min(lo, hourly[i].G), max(hi, hourly[i].G)
			if d.Anchor != nil && d.Anchor.G > 0 {
				p := (hourly[i].G/d.Anchor.G - 1) * 100
				hourly[i].VsAnchorPct = &p
			}
			if d.Used == "" && hourly[i].Fresh {
				d.Used = hourly[i].ID
			}
		}
		if d.Used == "" {
			d.Used = hourly[0].ID
		}
		if lo > 0 {
			d.Ratio = hi / lo
		}
		d.Level = "ok"
		switch {
		case len(hourly) >= 2 && d.Ratio >= 2.5:
			d.Level = "disagrees"
		case len(hourly) >= 2 && d.Ratio >= 1.5:
			d.Level = "differs"
		case len(hourly) == 1 && hourly[0].Readings >= 48 && d.Anchor != nil && d.Anchor.G > 0 && (hourly[0].G/d.Anchor.G >= 2 || hourly[0].G/d.Anchor.G <= 0.5):
			d.Level = "disagrees"
		}
		d.Sources = hourly
		out = append(out, d)
	}
	rank := map[string]int{"disagrees": 0, "differs": 1, "ok": 2}
	sort.SliceStable(out, func(i, j int) bool {
		if rank[out[i].Level] != rank[out[j].Level] {
			return rank[out[i].Level] < rank[out[j].Level]
		}
		if out[i].InUse != out[j].InUse {
			return out[i].InUse
		}
		return out[i].Ratio > out[j].Ratio
	})
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}
