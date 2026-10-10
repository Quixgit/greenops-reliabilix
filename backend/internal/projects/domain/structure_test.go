package domain

import (
	"math"
	"testing"
)

func sum(ps []ProjectCost, f func(ProjectCost) float64) float64 {
	var t float64
	for _, p := range ps {
		t += f(p)
	}
	return t
}

func find(ps []ProjectCost, id string) ProjectCost {
	for _, p := range ps {
		if p.ProjectID == id {
			return p
		}
	}
	return ProjectCost{}
}

func TestAllocateMovesCostWithoutCreatingAny(t *testing.T) {
	lines := []CostLine{
		{"platform", "Support", 100}, {"platform", "Compute", 400},
		{"web", "Compute", 50}, {"api", "Compute", 30},
	}
	rules := []AllocationRule{{SourceProjectID: "platform", ServiceName: "Support",
		Targets: []AllocationTarget{{"web", 3}, {"api", 1}}}}
	got := Allocate(lines, rules)
	if d := sum(got, func(p ProjectCost) float64 { return p.Total }) - 580; math.Abs(d) > 1e-9 {
		t.Errorf("totals must add up to the direct cost, off by %v", d)
	}
	if p := find(got, "platform"); p.Direct != 500 || p.AllocatedOut != 100 || p.Total != 400 {
		t.Errorf("platform: %+v", p)
	}
	if p := find(got, "web"); p.AllocatedIn != 75 || p.Total != 125 {
		t.Errorf("web: %+v", p)
	}
	if p := find(got, "api"); p.AllocatedIn != 25 || p.Total != 55 {
		t.Errorf("api: %+v", p)
	}
}

func TestSpecificRuleBeatsTheRuleForEverything(t *testing.T) {
	lines := []CostLine{{"shared", "Support", 100}, {"shared", "Compute", 200}}
	rules := []AllocationRule{
		{SourceProjectID: "shared", ServiceName: "", Targets: []AllocationTarget{{"a", 1}}},
		{SourceProjectID: "shared", ServiceName: "Support", Targets: []AllocationTarget{{"b", 1}}},
	}
	got := Allocate(lines, rules)
	if find(got, "b").AllocatedIn != 100 || find(got, "a").AllocatedIn != 200 {
		t.Errorf("Support must go to b and the rest to a: %+v", got)
	}
}

func TestSourceKeepsItsOwnShare(t *testing.T) {
	lines := []CostLine{{"shared", "Support", 100}}
	rules := []AllocationRule{{SourceProjectID: "shared", ServiceName: "Support", Targets: []AllocationTarget{{"shared", 1}, {"a", 1}}}}
	got := Allocate(lines, rules)
	if p := find(got, "shared"); p.AllocatedOut != 50 || p.Total != 50 {
		t.Errorf("shared: %+v", p)
	}
	if find(got, "a").Total != 50 {
		t.Errorf("a: %+v", find(got, "a"))
	}
}

func TestNoRulesNoChange(t *testing.T) {
	got := Allocate([]CostLine{{"a", "x", 10}, {"a", "y", 5}}, nil)
	if len(got) != 1 || got[0].Total != 15 || got[0].AllocatedIn != 0 {
		t.Errorf("%+v", got)
	}
}

func TestRuleValidate(t *testing.T) {
	ok := AllocationRule{Name: "Support", SourceProjectID: "p", Targets: []AllocationTarget{{"a", 1}, {"b", 2}}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]AllocationRule{
		"no name":      {SourceProjectID: "p", Targets: ok.Targets},
		"no source":    {Name: "x", Targets: ok.Targets},
		"no targets":   {Name: "x", SourceProjectID: "p"},
		"zero weight":  {Name: "x", SourceProjectID: "p", Targets: []AllocationTarget{{"a", 0}}},
		"nan weight":   {Name: "x", SourceProjectID: "p", Targets: []AllocationTarget{{"a", math.NaN()}}},
		"duplicate":    {Name: "x", SourceProjectID: "p", Targets: []AllocationTarget{{"a", 1}, {"a", 1}}},
		"long service": {Name: "x", SourceProjectID: "p", ServiceName: string(make([]rune, 201)), Targets: ok.Targets},
	}
	for name, r := range bad {
		if err := r.Validate(); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}
