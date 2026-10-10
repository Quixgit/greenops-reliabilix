package domain

import (
	"errors"
	"testing"
)

func f(v float64) *float64 { return &v }

func TestBudgetThresholds(t *testing.T) {
	conds := Conditions([]BudgetSignal{
		{ID: "a", Amount: 100, Spent: 79.9, Currency: "USD", Period: "monthly"},
		{ID: "b", Amount: 100, Spent: 80, Currency: "USD", Period: "monthly"},
		{ID: "c", Amount: 100, Spent: 150, Currency: "USD", Period: "monthly"},
		{ID: "d", Amount: 0, Spent: 5, Currency: "USD", Period: "monthly"},
	}, nil)
	if len(conds) != 2 {
		t.Fatalf("a is below the warning, d has no amount: got %d conditions", len(conds))
	}
	if conds[0].DedupeKey != "budget:b" || conds[0].Severity != Warning || *conds[0].Threshold != 80 {
		t.Errorf("b: %+v", conds[0])
	}
	if conds[1].Severity != Critical || *conds[1].Value != 150 {
		t.Errorf("c: %+v", conds[1])
	}
}

func TestConnectionConditions(t *testing.T) {
	conds := Conditions(nil, []ConnectionSignal{
		{ID: "1", ProjectID: "p", Provider: "aws", AccountRef: "123", SyncStatus: "error", LastError: "Access denied"},
		{ID: "2", ProjectID: "p", Provider: "gcp", AccountRef: "x", SyncStatus: "healthy", QualityStatus: "mismatch", MismatchDays: 3},
		{ID: "3", ProjectID: "p", Provider: "aws", AccountRef: "9", SyncStatus: "healthy", QualityStatus: "unchecked"},
	})
	if len(conds) != 2 || conds[0].Kind != KindConnectionError || conds[1].Kind != KindDataQuality {
		t.Fatalf("unchecked is not a problem; got %+v", conds)
	}
}

func TestReconcileOpensRefreshesAndResolves(t *testing.T) {
	live := []Alert{{ID: "1", DedupeKey: "budget:a"}, {ID: "2", DedupeKey: "connection:x"}}
	conds := []Condition{{DedupeKey: "budget:a"}, {DedupeKey: "budget:new"}}
	p := Reconcile(live, conds)
	if len(p.Open) != 1 || p.Open[0].DedupeKey != "budget:new" || len(p.Refresh) != 1 || len(p.Resolve) != 1 || p.Resolve[0] != "2" {
		t.Fatalf("%+v", p)
	}
}

func TestTransitions(t *testing.T) {
	for _, tc := range []struct {
		from, to Status
		ok       bool
	}{
		{New, Acknowledged, true}, {New, Muted, true}, {Acknowledged, Muted, true}, {Muted, New, true},
		{Acknowledged, New, false}, {New, Resolved, false}, {Resolved, New, false}, {Muted, Acknowledged, false},
	} {
		err := Transition(tc.from, tc.to)
		if (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrInvalidTransition)) {
			t.Errorf("%s -> %s: %v", tc.from, tc.to, err)
		}
	}
	_ = f
}
