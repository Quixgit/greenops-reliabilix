package application

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
)

type fxRepo struct {
	domain.Repository
	latest time.Time
	saved  []domain.FXRate
}

func (r *fxRepo) LatestRateDay(context.Context) (time.Time, error) { return r.latest, nil }
func (r *fxRepo) SaveRates(_ context.Context, x []domain.FXRate) error {
	r.saved = append(r.saved, x...)
	return nil
}

type fxSource struct{ since time.Time }

func (s *fxSource) Fetch(_ context.Context, since time.Time) ([]domain.FXRate, error) {
	s.since = since
	return []domain.FXRate{{Day: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), Currency: "USD", PerEUR: 1.12}}, nil
}

func TestRefreshRatesLoadsEverythingFirstThenOnlyTheLastWeekPlus(t *testing.T) {
	repo, src := &fxRepo{}, &fxSource{}
	if _, err := (Service{Repo: repo}).RefreshRates(context.Background(), src); err != nil || !src.since.IsZero() {
		t.Fatalf("first run must ask for all history: since=%v err=%v", src.since, err)
	}
	repo.latest = time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	if _, err := (Service{Repo: repo}).RefreshRates(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC); !src.since.Equal(want) {
		t.Fatalf("since = %v, want a week before the newest stored day (%v)", src.since, want)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("saved %d", len(repo.saved))
	}
}
