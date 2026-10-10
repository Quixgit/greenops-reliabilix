// Package application holds the Kubernetes energy use-cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
)

var ErrProjectNotFound = errors.New("project not found")

type Service struct {
	Repo      domain.Repository
	Intensity domain.IntensitySource
	Now       func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// IngestResult tells the agent what happened to a batch.
type IngestResult struct {
	Accepted      int `json:"accepted"`
	WithoutGrid   int `json:"without_grid_data"` // stored, carbon unknown until a reading exists
	MethodologyID string
}

// Ingest validates a whole batch first (one bad sample refuses it, so an agent never half-delivers), attaches the grid
// reading of each hour and stores the samples. The same hour sent again replaces the earlier value (idempotent).
func (s Service) Ingest(ctx context.Context, tenantID, projectID string, samples []domain.Sample) (IngestResult, error) {
	if len(samples) == 0 || len(samples) > domain.MaxSamplesPerRequest {
		return IngestResult{}, fmt.Errorf("%w: send 1 to %d samples", domain.ErrInvalidSample, domain.MaxSamplesPerRequest)
	}
	now := s.now()
	for i, sm := range samples {
		if err := sm.Validate(now); err != nil {
			return IngestResult{}, fmt.Errorf("sample %d: %w", i, err)
		}
	}
	if ok, err := s.Repo.ProjectExists(ctx, tenantID, projectID); err != nil {
		return IngestResult{}, err
	} else if !ok {
		return IngestResult{}, ErrProjectNotFound
	}
	pue := methodology.Current.PUE
	type key struct {
		region string
		at     time.Time
	}
	cache := map[key]*float64{}
	res := IngestResult{MethodologyID: methodology.Version}
	rows := make([]domain.Stored, 0, len(samples))
	for _, sm := range samples {
		k := key{sm.Region, sm.PeriodStart}
		g, seen := cache[k]
		if !seen {
			v, ok, err := s.Intensity.IntensityAt(ctx, sm.Region, sm.PeriodStart.Add(time.Hour))
			if err != nil {
				return IngestResult{}, err
			}
			if ok {
				g = &v
			}
			cache[k] = g
		}
		st := domain.Stored{Sample: sm, MethodologyVersion: methodology.Version}
		if g != nil {
			kg, _ := domain.Carbon(sm.EnergyKWh, pue, *g, true)
			st.Intensity, st.CarbonKg = g, &kg
		} else {
			res.WithoutGrid++
		}
		rows = append(rows, st)
	}
	if err := s.Repo.Upsert(ctx, tenantID, projectID, rows); err != nil {
		return IngestResult{}, err
	}
	res.Accepted = len(rows)
	observability.KubernetesSamples.Add(float64(res.Accepted))
	return res, nil
}

func (s Service) Workloads(ctx context.Context, tenantID string, project *string, cluster string, from, to time.Time, limit int) ([]domain.WorkloadRow, error) {
	return s.Repo.Workloads(ctx, tenantID, project, cluster, from, to, limit)
}

func (s Service) Clusters(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.ClusterRow, error) {
	return s.Repo.Clusters(ctx, tenantID, project, from, to)
}
