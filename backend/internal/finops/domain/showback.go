package domain

import (
	"sort"
)

// Showback is the cost of a window split by the values of one cloud tag key (ADR-0061): who spends what. Cost without a value for the key is
// reported as Untagged, never spread over the others, and Coverage says how much of the bill the key explains: a split that explains 20% of the
// money should not be read as the whole story.
type Showback struct {
	Tag      string  `json:"tag"`
	Currency string  `json:"currency"`
	HasData  bool    `json:"has_data"`
	Total    float64 `json:"total_cost"`
	Tagged   float64 `json:"tagged_cost"`
	Untagged float64 `json:"untagged_cost"`
	Coverage float64 `json:"coverage_pct"` // tagged / total, 0 to 100
	// TagsFrom is the first day (YYYY-MM-DD) any cost of this key is stored for: days before it count as untagged only because tags were not read then.
	TagsFrom string         `json:"tags_from,omitempty"`
	Lines    []ShowbackLine `json:"lines"`
}

// ShowbackLine is one value of the tag and what it cost, with its three biggest services.
type ShowbackLine struct {
	Value       string  `json:"value"`
	Cost        float64 `json:"cost"`
	Share       float64 `json:"share_pct"` // of the total bill
	TopServices []Line  `json:"top_services"`
}

// TagCost is one stored line already converted to the reporting currency: the value, the service and the cost.
type TagCost struct {
	Value   string
	Service string
	Cost    float64
}

// TagKeyInfo describes a tag key found in the window: how many values it has and how much of the bill carries it.
type TagKeyInfo struct {
	Key      string  `json:"key"`
	Values   int     `json:"values"`
	Coverage float64 `json:"coverage_pct"`
}

const topServices = 3

// BuildShowback folds converted lines into the split. total is the whole bill of the window in the same currency. A negative total cost
// (credits beyond spend) or a total of zero gives no shares. Lines are ordered by cost, biggest first; a value that nets to zero or less is
// kept (a credit is information) but sorts last.
func BuildShowback(tag, currency string, lines []TagCost, total float64) Showback {
	sb := Showback{Tag: tag, Currency: currency, Total: total, Lines: []ShowbackLine{}}
	type acc struct {
		cost float64
		svc  map[string]float64
	}
	byValue := map[string]*acc{}
	for _, l := range lines {
		a := byValue[l.Value]
		if a == nil {
			a = &acc{svc: map[string]float64{}}
			byValue[l.Value] = a
		}
		a.cost += l.Cost
		a.svc[l.Service] += l.Cost
		sb.Tagged += l.Cost
	}
	for v, a := range byValue {
		line := ShowbackLine{Value: v, Cost: a.cost, TopServices: []Line{}}
		if total > 0 {
			line.Share = a.cost / total * 100
		}
		svcs := make([]Line, 0, len(a.svc))
		for s, c := range a.svc {
			svcs = append(svcs, Line{Service: s, Cost: c})
		}
		sort.Slice(svcs, func(i, j int) bool {
			if svcs[i].Cost != svcs[j].Cost {
				return svcs[i].Cost > svcs[j].Cost
			}
			return svcs[i].Service < svcs[j].Service
		})
		if len(svcs) > topServices {
			svcs = svcs[:topServices]
		}
		line.TopServices = svcs
		sb.Lines = append(sb.Lines, line)
	}
	sort.Slice(sb.Lines, func(i, j int) bool {
		if sb.Lines[i].Cost != sb.Lines[j].Cost {
			return sb.Lines[i].Cost > sb.Lines[j].Cost
		}
		return sb.Lines[i].Value < sb.Lines[j].Value
	})
	sb.Untagged = total - sb.Tagged
	if sb.Untagged < 0 {
		sb.Untagged = 0 // the tagged lines are read a little apart from the total (rounding, a sync between): never a negative remainder
	}
	if total > 0 {
		sb.Coverage = sb.Tagged / total * 100
		if sb.Coverage > 100 {
			sb.Coverage = 100
		}
	}
	sb.HasData = len(sb.Lines) > 0 || total != 0
	return sb
}
