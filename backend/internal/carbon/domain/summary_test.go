package domain

import "testing"

func TestGate(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	th := Thresholds{CarbonWarnKg: f(10), CarbonBlockKg: f(20), CostWarn: f(50), CostBlock: f(100)}
	cases := []struct {
		carbon, cost float64
		want         Verdict
	}{
		{5, 10, Pass},
		{10, 50, Pass}, // equal to the threshold is allowed
		{15, 10, Warn},
		{5, 60, Warn},
		{25, 0, Block},
		{5, 150, Block},
		{15, 150, Block},  // BLOCK outranks WARN
		{-30, -500, Pass}, // reductions are never blocked
	}
	for _, c := range cases {
		if got, _ := Gate(c.carbon, c.cost, th); got != c.want {
			t.Errorf("Gate(%v, %v) = %s, want %s", c.carbon, c.cost, got, c.want)
		}
	}
	if v, r := Gate(1e9, 1e9, Thresholds{}); v != Pass || len(r) != 0 {
		t.Errorf("no thresholds must pass: %s %v", v, r)
	}
	if (Thresholds{}).Any() || !th.Any() {
		t.Error("Any() wrong")
	}
}

func TestComparable(t *testing.T) {
	cases := []struct {
		cur, prev, window int
		want              bool
	}{
		{30, 30, 30, true},
		{26, 25, 30, true}, // >= 80% of 30 days (24) on both sides
		{30, 6, 30, false}, // previous window barely populated: would show a fake +500%
		{6, 30, 30, false}, // new tenant: only a few days of data
		{0, 0, 30, false},
		{7, 7, 7, true},
		{3, 3, 0, false},
	}
	for _, c := range cases {
		if got := Comparable(c.cur, c.prev, c.window); got != c.want {
			t.Errorf("Comparable(%d, %d, %d) = %v, want %v", c.cur, c.prev, c.window, got, c.want)
		}
	}
}
