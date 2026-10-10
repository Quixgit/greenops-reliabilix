package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
)

func TestCalculateStoresUnderTheRequestedVersion(t *testing.T) {
	st := &fakeStore{stored: map[string]float64{"eu-central-1": 400}}
	svc, _ := newSvc(st, []domain.UsageDay{ec2Day()}, unitsSrc{}, &grid{}, domain.Thresholds{})
	if _, err := svc.CalculateWindowVersion(context.Background(), "t", "p", day, day.AddDate(0, 0, 1), methodology.Version); err != nil {
		t.Fatal(err)
	}
	if len(st.replaced) != 1 || st.replaced[0] != methodology.Version || st.saved[0].MethodologyVersion != methodology.Version {
		t.Fatalf("replaced %v, row version %q", st.replaced, st.saved[0].MethodologyVersion)
	}
	if _, err := svc.CalculateWindowVersion(context.Background(), "t", "p", day, day.AddDate(0, 0, 1), "RLX-NOPE"); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("an unknown version must be refused, got %v", err)
	}
	if len(st.replaced) != 1 {
		t.Error("nothing may be written for an unknown version")
	}
}

func TestRecalculateRefusesUnknownVersionBeforeQueueing(t *testing.T) {
	svc, q := newSvc(&fakeStore{}, nil, nil, &grid{}, domain.Thresholds{})
	if err := svc.RecalculateVersion(context.Background(), "t", "p", day, day.AddDate(0, 0, 1), "RLX-NOPE"); !errors.Is(err, ErrUnknownVersion) || len(q.tasks) != 0 {
		t.Fatalf("err=%v tasks=%v", err, q.tasks)
	}
	if err := svc.RecalculateVersion(context.Background(), "t", "p", day, day.AddDate(0, 0, 1), ""); err != nil || len(q.tasks) != 1 {
		t.Fatalf("active version must queue: %v %v", err, q.tasks)
	}
}

func TestCompareNeverEstimates(t *testing.T) {
	st := &fakeStore{totals: map[string]domain.Totals{methodology.Version: {CO2eKg: 10, Rows: 5, Days: 7}}}
	svc, _ := newSvc(st, nil, nil, &grid{}, domain.Thresholds{})
	from, to := day, day.AddDate(0, 0, 7)

	// Same version on both sides: same days, delta exactly zero.
	c, err := svc.Compare(context.Background(), "t", nil, from, to, methodology.Version, methodology.Version)
	if err != nil || c.DeltaCO2eKg == nil || *c.DeltaCO2eKg != 0 || !c.A.HasData {
		t.Fatalf("%+v %v", c, err)
	}

	// A version with no stored rows says so and produces no delta.
	st.totals["X"] = domain.Totals{}
	if _, err := svc.Compare(context.Background(), "t", nil, from, to, methodology.Version, "X"); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("X is not in the registry: %v", err)
	}
}

func TestComputeWithUsesTheGivenFactorsAndStampsTheirVersion(t *testing.T) {
	f := methodology.Current
	f.Version = "TEST-DOUBLE"
	f.CostBasedKWhPerUSD = map[string]float64{"compute": f.CostBasedKWhPerUSD["compute"] * 2, "other": 1}
	lookup := func(context.Context, string, time.Time) (domain.IntensityRef, bool, error) {
		return domain.IntensityRef{G: 400, Source: "test", Basis: domain.BasisDayMean}, true, nil
	}
	base, _, err := domain.Compute(context.Background(), []domain.UsageDay{ec2Day()}, lookup, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := domain.ComputeWith(f, context.Background(), []domain.UsageDay{ec2Day()}, lookup, nil)
	if err != nil {
		t.Fatal(err)
	}
	if other[0].MethodologyVersion != "TEST-DOUBLE" || base[0].MethodologyVersion != methodology.Version {
		t.Fatalf("versions: %q %q", other[0].MethodologyVersion, base[0].MethodologyVersion)
	}
	if got, want := other[0].EnergyKWh, base[0].EnergyKWh*2; got < want*0.999 || got > want*1.001 {
		t.Errorf("energy %v, want twice %v", got, base[0].EnergyKWh)
	}
}
