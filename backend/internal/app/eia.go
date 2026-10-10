package app

import (
	"context"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/eia"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// eiaSource computes the intensity of a United States balancing authority from EIA's hourly generation by fuel. It
// covers the cloud regions whose zone is one of the mapped balancing authorities (the zone map, including the
// operator's overrides, decides) and needs a free key.
type eiaSource struct {
	ints  *integrations.Store
	meter *apiusage.Meter
}

func (eiaSource) ID() string { return domain.ProviderEIA }

func (e eiaSource) respondent(ctx context.Context, region string) string {
	zones := electricitymaps.DefaultZones()
	if ov, err := electricitymaps.ParseOverrides(e.ints.Value(ctx, "electricitymaps", "zone_overrides")); err == nil {
		for k, v := range ov {
			zones[k] = v
		}
	}
	return eia.Respondents[strings.ToUpper(zones[region])]
}

func (e eiaSource) Covers(ctx context.Context, region string) bool {
	return e.ints.Value(ctx, "eia", "api_key") != "" && e.respondent(ctx, region) != ""
}

func (e eiaSource) Range(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c := eia.New(e.ints.Value(ctx, "eia", "api_key"))
	c.Allow = func(ctx context.Context) error { return e.meter.Allow(ctx, domain.ProviderEIA) }
	c.Done = func(ctx context.Context, failed bool) { e.meter.Record(ctx, domain.ProviderEIA, failed) }
	rs, err := c.Hourly(ctx, e.respondent(ctx, region), from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Intensity, 0, len(rs))
	for _, r := range rs {
		out = append(out, domain.Intensity{Region: region, GPerKWh: r.G, At: r.At, Measure: domain.MeasureAverage})
	}
	return out, nil
}

// Forecast: EIA publishes actuals only.
func (eiaSource) Forecast(context.Context, string, time.Time, time.Time) ([]domain.Intensity, error) {
	return nil, nil
}
