package plans

import (
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// A bigger plan never costs less and never allows less; Enterprise is by quote (no list price) and has no quotas.
func TestPlansGrowInPriceAndLimits(t *testing.T) {
	var prev Plan
	for i, p := range Catalog() {
		last := i == len(Catalog())-1
		if last != (p.PriceMonthlyUSD == nil) {
			t.Errorf("%s: only the last plan (Enterprise) is by quote", p.ID)
		}
		if i > 0 {
			if p.PriceMonthlyUSD != nil && *p.PriceMonthlyUSD <= *prev.PriceMonthlyUSD {
				t.Errorf("%s must cost more than %s", p.ID, prev.ID)
			}
			for k, v := range p.Limits {
				if v != 0 && v < prev.Limits[k] {
					t.Errorf("%s allows less %s than %s", p.ID, k, prev.ID)
				}
			}
		}
		prev = p
	}
	if f, _ := Lookup("free"); f.PriceMonthlyUSD == nil || *f.PriceMonthlyUSD != 0 {
		t.Error("Free costs nothing")
	}
}

// Every key a plan writes must be an operator-scope setting of the engine, and every value must pass its validation:
// a plan that the engine would reject is caught here, not when an operator assigns it.
func TestEveryPlanIsAcceptedByTheSettingsCatalog(t *testing.T) {
	cat := settings.Default()
	seen := map[string]bool{}
	for _, p := range Catalog() {
		if seen[p.ID] {
			t.Fatalf("duplicate plan %s", p.ID)
		}
		seen[p.ID] = true
		for k, raw := range p.Values() {
			d, ok := cat.Lookup(k)
			if !ok {
				t.Fatalf("plan %s writes unknown setting %s", p.ID, k)
			}
			if d.Scope != settings.ScopeOperator {
				t.Errorf("plan %s writes %s, which is not an operator setting", p.ID, k)
			}
			if _, _, err := d.Validate(raw); err != nil {
				t.Errorf("plan %s: %s = %s rejected: %v", p.ID, k, raw, err)
			}
		}
	}
	if _, ok := Lookup("free"); !ok {
		t.Error("free plan missing")
	}
}
