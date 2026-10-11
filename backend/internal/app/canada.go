package app

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/hydroquebec"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/ieso"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// zoneOf is the grid zone of a cloud region: the default map with the operator's overrides on top.
func zoneOf(ctx context.Context, ints *integrations.Store, region string) string {
	zones := electricitymaps.DefaultZones()
	if ov, err := electricitymaps.ParseOverrides(ints.Value(ctx, "electricitymaps", "zone_overrides")); err == nil {
		for k, v := range ov {
			zones[k] = v
		}
	}
	return zones[region]
}

// iesoSource computes the intensity of Ontario (zone CA-ON) from the IESO's public generation by fuel. No key; past data is
// available, so it fills the history of those regions.
type iesoSource struct {
	ints  *integrations.Store
	meter *apiusage.Meter
}

func (iesoSource) ID() string { return domain.ProviderIESO }

func (s iesoSource) Covers(ctx context.Context, region string) bool {
	return zoneOf(ctx, s.ints, region) == "CA-ON"
}

func (s iesoSource) Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := ieso.New()
	if base := s.ints.ValueOrDefault(ctx, "ieso", "base_url"); base != "" {
		c.BaseURL = base
	}
	c.Allow = func(ctx context.Context) error { return s.meter.Allow(ctx, domain.ProviderIESO) }
	c.Done = func(ctx context.Context, failed bool) { s.meter.Record(ctx, domain.ProviderIESO, failed) }
	rs, err := c.Hourly(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, Measure: domain.MeasureAverage})
	}
	return out, nil
}

// Forecast: the reports are actuals only.
func (iesoSource) Forecast(context.Context, string, time.Time, time.Time) ([]domain.Intensity, error) {
	return nil, nil
}

// hydroQuebecSource computes the intensity of Quebec (zone CA-QC) from Hydro-Québec's open generation data.
type hydroQuebecSource struct {
	ints  *integrations.Store
	meter *apiusage.Meter
}

func (hydroQuebecSource) ID() string { return domain.ProviderHydroQuebec }

func (s hydroQuebecSource) Covers(ctx context.Context, region string) bool {
	return zoneOf(ctx, s.ints, region) == "CA-QC"
}

func (s hydroQuebecSource) Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := hydroquebec.New()
	if base := s.ints.ValueOrDefault(ctx, "hydroquebec", "base_url"); base != "" {
		c.BaseURL = base
	}
	c.Allow = func(ctx context.Context) error { return s.meter.Allow(ctx, domain.ProviderHydroQuebec) }
	c.Done = func(ctx context.Context, failed bool) { s.meter.Record(ctx, domain.ProviderHydroQuebec, failed) }
	rs, err := c.Hourly(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, Measure: domain.MeasureAverage})
	}
	return out, nil
}

// Forecast: open data is actuals only.
func (hydroQuebecSource) Forecast(context.Context, string, time.Time, time.Time) ([]domain.Intensity, error) {
	return nil, nil
}
