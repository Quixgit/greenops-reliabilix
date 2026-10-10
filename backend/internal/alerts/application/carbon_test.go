package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
)

type carbonSrc struct {
	kg  float64
	err error
}

func (c carbonSrc) MonthToDate(context.Context, string, time.Time) (domain.CarbonSignal, error) {
	return domain.CarbonSignal{MonthToDateKg: c.kg}, c.err
}

// With the default settings the carbon limit is 0 (off): emissions alone never open an alert.
func TestCarbonAlertIsOffUntilALimitIsSet(t *testing.T) {
	r := &memRepo{}
	s := svc(r, budgets{}, conns{}, nil)
	s.Carbon = carbonSrc{kg: 5000}
	if err := s.Evaluate(context.Background(), "t"); err != nil {
		t.Fatal(err)
	}
	if len(r.opened) != 0 {
		t.Fatalf("no limit set, nothing may open: %+v", r.opened)
	}
}

func TestACarbonReadFailureLeavesCarbonAlertsAlone(t *testing.T) {
	r := &memRepo{live: []domain.Alert{{ID: "c", Kind: domain.KindCarbon, DedupeKey: "carbon:month"}}}
	s := svc(r, budgets{}, conns{}, nil)
	s.Carbon = carbonSrc{err: errors.New("carbon down")}
	if err := s.Evaluate(context.Background(), "t"); err != nil {
		t.Fatal(err)
	}
	if len(r.resolved) != 0 {
		t.Fatalf("an unreadable carbon source must not resolve its alert: %v", r.resolved)
	}
}
