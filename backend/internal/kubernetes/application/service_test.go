package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
)

type memRepo struct {
	domain.Repository
	project bool
	stored  []domain.Stored
}

func (m *memRepo) ProjectExists(context.Context, string, string) (bool, error) { return m.project, nil }
func (m *memRepo) Upsert(_ context.Context, _, _ string, rows []domain.Stored) error {
	m.stored = append(m.stored, rows...)
	return nil
}

type grid map[string]float64

func (g grid) IntensityAt(_ context.Context, region string, _ time.Time) (float64, bool, error) {
	v, ok := g[region]
	return v, ok, nil
}

var at = time.Date(2026, 10, 10, 12, 30, 0, 0, time.UTC)

func sample(region string, hour int) domain.Sample {
	return domain.Sample{Cluster: "c", Namespace: "n", Workload: "w", Kind: "deployment",
		PeriodStart: time.Date(2026, 10, 10, hour, 0, 0, 0, time.UTC), EnergyKWh: 2, Region: region}
}

func svc(r *memRepo, g grid) Service { return Service{Repo: r, Intensity: g, Now: func() time.Time { return at }} }

func TestIngestAttachesCarbonOnlyWhereTheGridIsKnown(t *testing.T) {
	r := &memRepo{project: true}
	res, err := svc(r, grid{"eu-central-1": 400}).Ingest(context.Background(), "t", "p", []domain.Sample{sample("eu-central-1", 9), sample("mars-1", 9)})
	if err != nil || res.Accepted != 2 || res.WithoutGrid != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if r.stored[0].CarbonKg == nil || *r.stored[0].CarbonKg < 0.9 || *r.stored[0].CarbonKg > 0.92 {
		t.Fatalf("2 kWh x PUE 1.135 x 400 g = 0.908 kg, got %v", r.stored[0].CarbonKg)
	}
	if r.stored[1].CarbonKg != nil || r.stored[1].Intensity != nil {
		t.Fatalf("no grid reading: carbon must stay unknown, got %+v", r.stored[1])
	}
}

func TestOneBadSampleRefusesTheWholeBatch(t *testing.T) {
	r := &memRepo{project: true}
	bad := sample("eu-central-1", 9)
	bad.EnergyKWh = 9e9
	_, err := svc(r, grid{"eu-central-1": 400}).Ingest(context.Background(), "t", "p", []domain.Sample{sample("eu-central-1", 8), bad})
	if !errors.Is(err, domain.ErrInvalidSample) || len(r.stored) != 0 {
		t.Fatalf("a half-delivered batch must not exist: err=%v stored=%d", err, len(r.stored))
	}
}

func TestUnknownProjectAndBatchSizeAreRefused(t *testing.T) {
	if _, err := svc(&memRepo{project: false}, grid{}).Ingest(context.Background(), "t", "p", []domain.Sample{sample("eu-central-1", 9)}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("unknown project: %v", err)
	}
	r := &memRepo{project: true}
	if _, err := svc(r, grid{}).Ingest(context.Background(), "t", "p", nil); !errors.Is(err, domain.ErrInvalidSample) {
		t.Fatalf("empty batch: %v", err)
	}
	big := make([]domain.Sample, domain.MaxSamplesPerRequest+1)
	for i := range big {
		big[i] = sample("eu-central-1", 9)
	}
	if _, err := svc(r, grid{}).Ingest(context.Background(), "t", "p", big); !errors.Is(err, domain.ErrInvalidSample) {
		t.Fatalf("oversized batch: %v", err)
	}
}
