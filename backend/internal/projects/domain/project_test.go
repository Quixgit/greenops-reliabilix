package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	if _, err := New("t", "  api  ", "request", time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{{"", "u"}, {strings.Repeat("x", 121), "u"}, {"n", ""}} {
		if _, err := New("t", c[0], c[1], time.Now()); err == nil {
			t.Errorf("%q/%q accepted", c[0], c[1])
		}
	}
}

func TestFunctionalUnitsValidate(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse(time.DateOnly, s); return v }
	ok := FunctionalUnits{PeriodStart: d("2026-09-01"), PeriodEnd: d("2026-09-30"), Units: 1_000_000}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]FunctionalUnits{
		"inverted": {PeriodStart: d("2026-09-30"), PeriodEnd: d("2026-09-01"), Units: 1},
		"zero":     {PeriodStart: d("2026-09-01"), PeriodEnd: d("2026-09-02"), Units: 0},
		"too long": {PeriodStart: d("2024-01-01"), PeriodEnd: d("2026-01-01"), Units: 1},
		"negative": {PeriodStart: d("2026-09-01"), PeriodEnd: d("2026-09-02"), Units: -3},
	} {
		if f.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestPolicyValidate(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	p := Policy{AllowedRegions: []string{" EU-West-1 ", "eu-west-1", "eu-north-1"}, CICarbonWarnKg: f(10), CICarbonBlockKg: f(20)}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(p.AllowedRegions) != 2 || p.AllowedRegions[0] != "eu-west-1" {
		t.Errorf("not normalized/deduped: %v", p.AllowedRegions)
	}
	bad := []Policy{
		{AllowedRegions: []string{"not a region!"}},
		{CICarbonWarnKg: f(30), CICarbonBlockKg: f(20)},
		{CICostBlock: f(-1)},
	}
	for i, b := range bad {
		if b.Validate() == nil {
			t.Errorf("bad policy %d accepted", i)
		}
	}
}
