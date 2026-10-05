// Package application holds the carbon use-cases.
package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
)

// Service resolves grid intensity (cached: the dashboard must never trigger a
// provider call per request) and runs the methodology.
type Service struct {
	Provider domain.CarbonDataProvider
	TTL      time.Duration

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	v   domain.Intensity
	exp time.Time
}

func (s *Service) intensity(ctx context.Context, region string) (domain.Intensity, error) {
	s.mu.Lock()
	if c, ok := s.cache[region]; ok && time.Now().Before(c.exp) {
		s.mu.Unlock()
		return c.v, nil
	}
	s.mu.Unlock()
	if s.Provider == nil {
		return domain.Intensity{}, errors.New("carbon: no grid data provider configured")
	}
	v, err := s.Provider.GetIntensity(ctx, region)
	if err != nil {
		observability.CarbonAPIErrors.Inc()
		return domain.Intensity{}, err
	}
	ttl := s.TTL
	if ttl == 0 {
		ttl = time.Hour
	}
	s.mu.Lock()
	if s.cache == nil {
		s.cache = map[string]cached{}
	}
	s.cache[region] = cached{v: v, exp: time.Now().Add(ttl)}
	s.mu.Unlock()
	return v, nil
}

// Calculate converts usage in a region to energy and CO2e.
func (s *Service) Calculate(ctx context.Context, kind domain.UsageKind, amount float64, region string) (domain.Result, error) {
	in, err := s.intensity(ctx, region)
	if err != nil {
		return domain.Result{}, err
	}
	res, err := domain.Calculate(domain.Input{Kind: kind, Amount: amount, GridIntensityGPerKWh: in.GPerKWh})
	if err == nil {
		observability.CarbonCalculations.Inc()
	}
	return res, err
}
