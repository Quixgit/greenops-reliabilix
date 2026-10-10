package domain

import (
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

var at = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func ct(day, cur string, v float64) focus.ControlTotal {
	return focus.ControlTotal{Day: day, Currency: cur, Billed: v}
}

func TestReconcileMatchesWithinRoundingTolerance(t *testing.T) {
	q := Reconcile(
		[]focus.ControlTotal{ct("2026-09-01", "USD", 1000), ct("2026-09-02", "USD", 0.01)},
		[]focus.ControlTotal{ct("2026-09-01", "USD", 1004.9), ct("2026-09-02", "USD", 0)}, // 0.49% and a cent
		at)
	if q.Status != QualityOK || q.CheckedDays != 2 || q.MismatchDays != 0 {
		t.Errorf("quality = %+v", q)
	}
	if q.ProviderTotal["USD"] != 1000.01 || q.StoredTotal["USD"] != 1004.9 {
		t.Errorf("totals = %v / %v", q.ProviderTotal, q.StoredTotal)
	}
}

func TestReconcileCatchesAMissingDayAndAWrongAmount(t *testing.T) {
	q := Reconcile(
		[]focus.ControlTotal{ct("2026-09-01", "USD", 100), ct("2026-09-02", "USD", 200), ct("2026-09-03", "USD", 300)},
		[]focus.ControlTotal{ct("2026-09-01", "USD", 100), ct("2026-09-03", "USD", 150)}, // day 2 missing, day 3 half
		at)
	if q.Status != QualityMismatch || q.MismatchDays != 2 || q.MaxDiffPct != 100 {
		t.Fatalf("quality = %+v", q)
	}
	if got := q.MismatchedOn; len(got) != 2 || got[0] != "2026-09-02" || got[1] != "2026-09-03" {
		t.Errorf("mismatched days = %v", got)
	}
	first, ok := q.EarliestMismatch()
	if !ok || first.Format(time.DateOnly) != "2026-09-02" {
		t.Errorf("earliest = %v %v", first, ok)
	}
}

func TestReconcileTreatsCurrenciesSeparatelyAndNegativeDays(t *testing.T) {
	q := Reconcile(
		[]focus.ControlTotal{ct("2026-09-01", "USD", -50), ct("2026-09-01", "EUR", 10)},
		[]focus.ControlTotal{ct("2026-09-01", "USD", -50), ct("2026-09-01", "EUR", 7)}, at)
	if q.Status != QualityMismatch || q.MismatchDays != 1 {
		t.Errorf("a EUR shortfall must not hide behind matching USD: %+v", q)
	}
	if ok := Reconcile([]focus.ControlTotal{ct("2026-09-01", "USD", -50)}, []focus.ControlTotal{ct("2026-09-01", "USD", -50.2)}, at); ok.Status != QualityOK {
		t.Errorf("credits (negative days) within tolerance must match: %+v", ok)
	}
}

func TestReconcileWithoutAControlIsUncheckedNotOK(t *testing.T) {
	q := Reconcile(nil, []focus.ControlTotal{ct("2026-09-01", "USD", 5)}, at)
	if q.Status != QualityUnchecked || q.Note == "" {
		t.Errorf("no control totals must never read as ok: %+v", q)
	}
	if u := Unchecked("why", at); u.Status != QualityUnchecked || u.Note != "why" || !u.CheckedAt.Equal(at) {
		t.Errorf("Unchecked = %+v", u)
	}
}

func TestReconcileListsAtMostTenDays(t *testing.T) {
	var control []focus.ControlTotal
	for d := 1; d <= 20; d++ {
		control = append(control, ct(time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC).Format(time.DateOnly), "USD", 100))
	}
	q := Reconcile(control, nil, at)
	if q.MismatchDays != 20 || len(q.MismatchedOn) != 10 || q.MismatchedOn[0] != "2026-09-01" {
		t.Errorf("quality = days %d listed %v", q.MismatchDays, q.MismatchedOn)
	}
}
