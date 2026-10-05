package domain

import (
	"testing"
	"time"
)

func TestPeriodBounds(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	for period, want := range map[string][2]string{
		"monthly":   {"2026-11-01", "2026-12-01"},
		"quarterly": {"2026-10-01", "2027-01-01"},
		"yearly":    {"2026-01-01", "2027-01-01"},
	} {
		s, e := PeriodBounds(period, now)
		if s.Format(time.DateOnly) != want[0] || e.Format(time.DateOnly) != want[1] {
			t.Errorf("%s: %v..%v, want %v", period, s, e, want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC) // 10 of 30 days elapsed
	b := Budget{Amount: 1000, Currency: "USD", Period: "monthly"}
	cases := []struct {
		spent float64
		state string
	}{{100, "ok"}, {400, "at_risk"}, {850, "at_risk"}, {1200, "exceeded"}}
	for _, c := range cases {
		if got := Evaluate(b, c.spent, now).State; got != c.state {
			t.Errorf("spent %v: %s, want %s", c.spent, got, c.state)
		}
	}
	if s := Evaluate(b, 100, now); s.Projected < 290 || s.Projected > 310 {
		t.Errorf("projection = %v, want ~300", s.Projected)
	}
}

func TestBudgetValidate(t *testing.T) {
	if (Budget{Amount: 1, Currency: "USD", Period: "monthly"}).Validate() != nil {
		t.Error("valid budget rejected")
	}
	for _, b := range []Budget{{Amount: -1, Currency: "USD", Period: "monthly"}, {Amount: 1, Currency: "US", Period: "monthly"}, {Amount: 1, Currency: "USD", Period: "weekly"}} {
		if b.Validate() == nil {
			t.Errorf("%+v accepted", b)
		}
	}
}
