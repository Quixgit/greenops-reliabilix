package app

import (
	"context"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/entsoe"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/ons"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// entsoeSource computes the intensity of a European area from the ENTSO-E Transparency Platform's actual generation per
// production type. It needs the platform's security token (free) and covers the cloud regions whose zone, or whose zone's
// country, is one the connector knows (the zone map, including the operator's overrides, decides).
type entsoeSource struct {
	ints  *integrations.Store
	meter *apiusage.Meter
}

func (entsoeSource) ID() string { return domain.ProviderENTSOE }

func (s entsoeSource) area(ctx context.Context, region string) string {
	zone := strings.ToUpper(zoneOf(ctx, s.ints, region))
	if a, ok := entsoe.Domains[zone]; ok {
		return a
	}
	return entsoe.Domains[strings.SplitN(zone, "-", 2)[0]]
}

func (s entsoeSource) Covers(ctx context.Context, region string) bool {
	return s.ints.Value(ctx, "entsoe", "api_token") != "" && s.area(ctx, region) != ""
}

func (s entsoeSource) Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := entsoe.New(s.ints.Value(ctx, "entsoe", "api_token"))
	if base := s.ints.ValueOrDefault(ctx, "entsoe", "base_url"); base != "" {
		c.BaseURL = base
	}
	c.Allow = func(ctx context.Context) error { return s.meter.Allow(ctx, domain.ProviderENTSOE) }
	c.Done = func(ctx context.Context, failed bool) { s.meter.Record(ctx, domain.ProviderENTSOE, failed) }
	rs, err := c.Hourly(ctx, s.area(ctx, region), from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, Measure: domain.MeasureAverage})
	}
	return out, nil
}

// Forecast: the platform's actual generation is actuals only.
func (entsoeSource) Forecast(context.Context, string, time.Time, time.Time) ([]domain.Intensity, error) {
	return nil, nil
}

// onsSource computes the intensity of a Brazilian subsystem from the ONS open energy balance. No key; the year files carry
// the history, so it fills the past of those regions.
type onsSource struct {
	ints  *integrations.Store
	meter *apiusage.Meter
}

func (onsSource) ID() string { return domain.ProviderONS }

func (s onsSource) subsystem(ctx context.Context, region string) string {
	return ons.Subsystems[strings.ToUpper(zoneOf(ctx, s.ints, region))]
}

func (s onsSource) Covers(ctx context.Context, region string) bool {
	return s.subsystem(ctx, region) != ""
}

func (s onsSource) Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := ons.New()
	if base := s.ints.ValueOrDefault(ctx, "ons", "base_url"); base != "" {
		c.BaseURL = base
	}
	c.Allow = func(ctx context.Context) error { return s.meter.Allow(ctx, domain.ProviderONS) }
	c.Done = func(ctx context.Context, failed bool) { s.meter.Record(ctx, domain.ProviderONS, failed) }
	rs, err := c.Hourly(ctx, s.subsystem(ctx, region), from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, Measure: domain.MeasureAverage})
	}
	return out, nil
}

// Forecast: the energy balance is actuals only.
func (onsSource) Forecast(context.Context, string, time.Time, time.Time) ([]domain.Intensity, error) {
	return nil, nil
}
