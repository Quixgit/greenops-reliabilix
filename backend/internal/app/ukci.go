package app

import (
	"context"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/ukci"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// ukciSource adapts the UK Carbon Intensity client to the carbon module. It needs no key; it covers the cloud regions
// whose zone is Great Britain (the zone map, including the operator's overrides, decides).
type ukciSource struct {
	ints  *integrations.Store
	meter *apiusage.Meter
}

func (ukciSource) ID() string { return domain.ProviderUKCI }

func (u ukciSource) zone(ctx context.Context, region string) string {
	zones := electricitymaps.DefaultZones()
	if ov, err := electricitymaps.ParseOverrides(u.ints.Value(ctx, "electricitymaps", "zone_overrides")); err == nil {
		for k, v := range ov {
			zones[k] = v
		}
	}
	return zones[region]
}

func (u ukciSource) Covers(ctx context.Context, region string) bool {
	return strings.EqualFold(strings.SplitN(u.zone(ctx, region), "-", 2)[0], "GB")
}

func (u ukciSource) Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := ukci.New()
	if base := u.ints.ValueOrDefault(ctx, "uk_carbon_intensity", "base_url"); base != "" {
		c.BaseURL = base
	}
	c.Allow = func(ctx context.Context) error { return u.meter.Allow(ctx, domain.ProviderUKCI) }
	c.Done = func(ctx context.Context, failed bool) { u.meter.Record(ctx, domain.ProviderUKCI, failed) }
	rs, err := c.Range(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, Measure: domain.MeasureAverage})
	}
	return out, nil
}

// Forecast returns the forecast half-hours for planning; they are stored as forecasts, never as readings.
func (u ukciSource) Forecast(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := ukci.New()
	if base := u.ints.ValueOrDefault(ctx, "uk_carbon_intensity", "base_url"); base != "" {
		c.BaseURL = base
	}
	c.Allow = func(ctx context.Context) error { return u.meter.Allow(ctx, domain.ProviderUKCI) }
	c.Done = func(ctx context.Context, failed bool) { u.meter.Record(ctx, domain.ProviderUKCI, failed) }
	rs, err := c.Forecast(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, IsForecast: true, Measure: domain.MeasureAverage})
	}
	return out, nil
}
