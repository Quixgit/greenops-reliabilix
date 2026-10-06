package application

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
)

// fakeRepo implements only what IngestRightsizing uses; the embedded nil interface panics on anything else,
// which keeps the test honest about what the use-case touches.
type fakeRepo struct {
	domain.Repository
	intensity map[string]float64
	byPrint   map[string]domain.Recommendation
}

func (f *fakeRepo) Intensities(context.Context) (map[string]float64, error) { return f.intensity, nil }
func (f *fakeRepo) Insert(_ context.Context, _ string, r domain.Recommendation) (bool, error) {
	if _, dup := f.byPrint[r.Fingerprint]; dup {
		return false, nil
	}
	f.byPrint[r.Fingerprint] = r
	return true, nil
}

func newService(repo *fakeRepo) Service {
	return Service{Repo: repo, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }}
}

func TestIngestRightsizingStoresOnceAndUsesProviderShapes(t *testing.T) {
	repo := &fakeRepo{intensity: map[string]float64{"eu-central-1": 400}, byPrint: map[string]domain.Recommendation{}}
	s := newService(repo)
	findings := []domain.RightsizingFinding{
		{ResourceID: "i-1", Region: "eu-central-1", CurrentType: "m5.2xlarge", TargetType: "m5.large"}, // shapes from ec2spec
		{ResourceID: "i-2", Region: "eu-central-1", CurrentType: "weird.type"},                         // unknown shape: skipped
	}
	n, err := s.IngestRightsizing(context.Background(), "t", "p", findings)
	if err != nil || n != 1 {
		t.Fatalf("created %d, err %v; want 1 (the unknown shape is skipped)", n, err)
	}
	if again, _ := s.IngestRightsizing(context.Background(), "t", "p", findings); again != 0 {
		t.Fatalf("a second ingest created %d duplicates", again)
	}
	for _, r := range repo.byPrint {
		if r.ProjectID != "p" || r.Type != domain.Rightsizing || r.CarbonReductionKg <= 0 || r.Provider != "aws" {
			t.Errorf("stored recommendation wrong: %+v", r)
		}
	}
}

func TestIngestRightsizingWithNoFindingsDoesNotTouchTheRepo(t *testing.T) {
	s := newService(&fakeRepo{}) // a nil map would panic if Insert or Intensities were reached
	if n, err := s.IngestRightsizing(context.Background(), "t", "p", nil); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
