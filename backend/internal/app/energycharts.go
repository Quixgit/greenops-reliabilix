package app

import (
	"context"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/energycharts"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// energyChartsSource computes the intensity of a European country from Energy-Charts' public generation by type. It
// needs no key and covers the cloud regions whose zone is in a mapped country (the zone map, including the operator's
// overrides, decides). Past data is available, which makes it the free way to fill the history of those regions.
type energyChartsSource struct {
	ints  *integrations.Store
	meter *apiusage.Meter
}

func (energyChartsSource) ID() string { return domain.ProviderEnergyCharts }

func (e energyChartsSource) country(ctx context.Context, region string) string {
	zones := electricitymaps.DefaultZones()
	if ov, err := electricitymaps.ParseOverrides(e.ints.Value(ctx, "electricitymaps", "zone_overrides")); err == nil {
		for k, v := range ov {
			zones[k] = v
		}
	}
	return energycharts.Countries[strings.ToUpper(strings.SplitN(zones[region], "-", 2)[0])]
}

func (e energyChartsSource) Covers(ctx context.Context, region string) bool {
	return e.country(ctx, region) != ""
}

func (e energyChartsSource) Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := energycharts.New()
	if base := e.ints.ValueOrDefault(ctx, "energy_charts", "base_url"); base != "" {
		c.BaseURL = base
	}
	c.Allow = func(ctx context.Context) error { return e.meter.Allow(ctx, domain.ProviderEnergyCharts) }
	c.Done = func(ctx context.Context, failed bool) { e.meter.Record(ctx, domain.ProviderEnergyCharts, failed) }
	rs, err := c.Hourly(ctx, e.country(ctx, region), from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, Measure: domain.MeasureAverage})
	}
	return out, nil
}

// Forecast: Energy-Charts' generation endpoint is actuals only.
func (energyChartsSource) Forecast(context.Context, string, time.Time, time.Time) ([]domain.Intensity, error) {
	return nil, nil
}
