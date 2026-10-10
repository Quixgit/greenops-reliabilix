package application

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
)

type verifyRepo struct {
	domain.Repository
	recs    []domain.Recommendation
	before  []domain.DayPoint
	after   []domain.DayPoint
	saved   map[string]domain.Verification
	scopeOf string
}

func (v *verifyRepo) AppliedAwaitingVerification(context.Context, string, string) ([]domain.Recommendation, error) {
	return v.recs, nil
}
func (v *verifyRepo) DailySeries(_ context.Context, _, _, service string, from, _ time.Time) ([]domain.DayPoint, string, error) {
	v.scopeOf = service
	if from.Before(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		return v.before, "USD", nil
	}
	return v.after, "USD", nil
}
func (v *verifyRepo) SaveVerification(_ context.Context, _, id string, ver domain.Verification, _ string) error {
	v.saved[id] = ver
	return nil
}

func days(from time.Time, n int, cost, kg float64) []domain.DayPoint {
	out := make([]domain.DayPoint, n)
	for i := range out {
		out[i] = domain.DayPoint{Day: from.AddDate(0, 0, i), Cost: cost, CarbonKg: kg}
	}
	return out
}

func TestVerifyAppliedWaitsMeasuresAndGivesUp(t *testing.T) {
	applied := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	rec := domain.Recommendation{ID: "r1", Status: domain.Applied, AppliedAt: &applied, ServiceName: "Amazon EC2", CarbonReductionKg: 30}
	repo := &verifyRepo{recs: []domain.Recommendation{rec}, saved: map[string]domain.Verification{},
		before: days(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), 14, 10, 5),
		after:  days(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC), 14, 10, 4)}
	now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	s := Service{Repo: repo, Now: func() time.Time { return now }}

	// The after window (to 19 Sep) is still open: nothing is measured, nothing is saved.
	if n, err := s.VerifyApplied(context.Background(), "t", "p"); err != nil || n != 0 || len(repo.saved) != 0 {
		t.Fatalf("too early: n=%d err=%v saved=%d", n, err, len(repo.saved))
	}

	now = time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	n, err := s.VerifyApplied(context.Background(), "t", "p")
	if err != nil || n != 1 {
		t.Fatalf("window complete: n=%d err=%v", n, err)
	}
	if repo.scopeOf != "Amazon EC2" {
		t.Errorf("series must be scoped to the recommendation's service, got %q", repo.scopeOf)
	}
	if got := repo.saved["r1"]; got.Outcome != domain.Realized {
		t.Errorf("outcome %s, want realized", got.Outcome)
	}

	// Billing data never arrived: stays unmeasured until the give-up date, then is closed as inconclusive.
	repo.saved = map[string]domain.Verification{}
	repo.after = nil
	if n, _ := s.VerifyApplied(context.Background(), "t", "p"); n != 0 || len(repo.saved) != 0 {
		t.Fatalf("missing data must wait, got n=%d", n)
	}
	now = applied.AddDate(0, 0, 61)
	if n, _ := s.VerifyApplied(context.Background(), "t", "p"); n != 1 || repo.saved["r1"].Outcome != domain.Inconclusive {
		t.Fatalf("expired: n=%d saved=%+v", n, repo.saved)
	}
}
