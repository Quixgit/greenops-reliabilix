package domain

// UnitEconomics: what one unit of the customer's work costs and emits, project by project and month by month (ADR-0053). The unit is the
// project's own functional unit (requests, orders, customers ...), reported by the customer; only the days with units reported count, so a
// month with a gap in the reporting is never divided by units it does not have.

// UnitMonth is one month of one project.
type UnitMonth struct {
	Month          string   `json:"month"` // 2026-10
	Days           int      `json:"days"`  // days of the month with units reported
	Units          float64  `json:"units"`
	Cost           float64  `json:"cost"`
	CarbonG        float64  `json:"carbon_g"`
	CostPerUnit    *float64 `json:"cost_per_unit"`
	CarbonGPerUnit *float64 `json:"carbon_g_per_unit"`
}

// UnitProject is the series of one project and how the latest month compares with the one before.
type UnitProject struct {
	ProjectID string      `json:"project_id"`
	Name      string      `json:"name"`
	Unit      string      `json:"unit"`
	Months    []UnitMonth `json:"months"`
	// CostChangePct and CarbonChangePct compare the two latest months that both have at least 7 days of units; nil when there is no pair.
	CostChangePct   *float64 `json:"cost_change_pct"`
	CarbonChangePct *float64 `json:"carbon_change_pct"`
}

// UnitEconomics is the report: one currency (the reporting one), one series per project that reported units.
type UnitEconomics struct {
	Currency string        `json:"currency"`
	Projects []UnitProject `json:"projects"`
}

// MinDaysForChange is how many reported days a month needs to be compared with another.
const MinDaysForChange = 7

// Finish fills the per-unit figures and the change between the latest comparable months.
func (u *UnitEconomics) Finish() {
	for i := range u.Projects {
		p := &u.Projects[i]
		for j := range p.Months {
			m := &p.Months[j]
			if m.Units > 0 {
				c, g := m.Cost/m.Units, m.CarbonG/m.Units
				m.CostPerUnit, m.CarbonGPerUnit = &c, &g
			}
		}
		var last, prev *UnitMonth
		for j := len(p.Months) - 1; j >= 0; j-- {
			m := &p.Months[j]
			if m.Days < MinDaysForChange || m.CostPerUnit == nil {
				continue
			}
			if last == nil {
				last = m
			} else {
				prev = m
				break
			}
		}
		if last != nil && prev != nil && *prev.CostPerUnit > 0 {
			c := (*last.CostPerUnit / *prev.CostPerUnit - 1) * 100
			p.CostChangePct = &c
			if *prev.CarbonGPerUnit > 0 {
				g := (*last.CarbonGPerUnit / *prev.CarbonGPerUnit - 1) * 100
				p.CarbonChangePct = &g
			}
		}
	}
}
