package app

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/climatetrace"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/ember"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// inventoryAnnual derives, for a cloud region's country and a year, the direct-emission intensity of electricity: the power
// sector's emissions in the Climate TRACE inventory divided by the generation Ember publishes. It needs the Ember key (for the
// generation) and nothing else. It is a reference to hold hourly sources against, not a calculation input.
type inventoryAnnual struct {
	ints  *integrations.Store
	meter *apiusage.Meter

	mu    sync.Mutex
	cache map[string]invCached // country + year -> figure, valid for an hour (one refresh run)
}

type invCached struct {
	at time.Time
	g  float64
	ok bool
}

func (a *inventoryAnnual) Annual(ctx context.Context, region string, fromYear, toYear int) ([]domain.AnnualRow, error) {
	key := a.ints.Value(ctx, "ember", "api_key")
	if key == "" {
		return nil, domain.ErrNotConfigured
	}
	code, ok := ember.CountryOf(zoneOf(ctx, a.ints, region))
	if !ok {
		return nil, nil
	}
	var out []domain.AnnualRow
	for y := fromYear; y <= toYear; y++ {
		ck := code + "/" + strconv.Itoa(y)
		a.mu.Lock()
		if a.cache == nil {
			a.cache = map[string]invCached{}
		}
		c, hit := a.cache[ck]
		a.mu.Unlock()
		if !hit || time.Since(c.at) > time.Hour {
			g, ok, err := a.figure(ctx, key, code, y)
			if err != nil {
				return out, err
			}
			c = invCached{at: time.Now(), g: g, ok: ok}
			a.mu.Lock()
			a.cache[ck] = c
			a.mu.Unlock()
		}
		if c.ok {
			out = append(out, domain.AnnualRow{Year: y, Provider: domain.ProviderClimateTrace, G: c.g})
		}
	}
	return out, nil
}

// CountriesAnnual is the figure of every country with both an inventory and a generation for the year: two requests in all.
func (a *inventoryAnnual) CountriesAnnual(ctx context.Context, year int) ([]domain.CountryReading, error) {
	key := a.ints.Value(ctx, "ember", "api_key")
	if key == "" {
		return nil, domain.ErrNotConfigured
	}
	em := ember.New(key)
	em.Allow = func(ctx context.Context) error { return a.meter.Allow(ctx, "ember") }
	em.Done = func(ctx context.Context, failed bool) { a.meter.Record(ctx, "ember", failed) }
	gen, err := em.GenerationAllTWh(ctx, year)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(gen))
	for c := range gen {
		codes = append(codes, c)
	}
	ct := climatetrace.New()
	if base := a.ints.ValueOrDefault(ctx, "climatetrace", "base_url"); base != "" {
		ct.BaseURL = base
	}
	ct.Allow = func(ctx context.Context) error { return a.meter.Allow(ctx, domain.ProviderClimateTrace) }
	ct.Done = func(ctx context.Context, failed bool) { a.meter.Record(ctx, domain.ProviderClimateTrace, failed) }
	tonnes, err := ct.PowerTonnes(ctx, codes, year)
	if err != nil {
		return nil, err
	}
	var out []domain.CountryReading
	for code, t := range tonnes {
		twh, ok := gen[code]
		if !ok {
			continue
		}
		if g := t / (twh * 1000); g >= 1 && g <= 1500 {
			out = append(out, domain.CountryReading{Code: code, Month: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), G: g})
		}
	}
	return out, nil
}

// figure is tonnes of CO2e / (TWh x 1000) = gCO2e/kWh. ok=false when either side has no figure for that year.
func (a *inventoryAnnual) figure(ctx context.Context, key, country string, year int) (float64, bool, error) {
	ct := climatetrace.New()
	if base := a.ints.ValueOrDefault(ctx, "climatetrace", "base_url"); base != "" {
		ct.BaseURL = base
	}
	ct.Allow = func(ctx context.Context) error { return a.meter.Allow(ctx, domain.ProviderClimateTrace) }
	ct.Done = func(ctx context.Context, failed bool) { a.meter.Record(ctx, domain.ProviderClimateTrace, failed) }
	tonnes, err := ct.PowerTonnes(ctx, []string{strings.ToUpper(country)}, year)
	if err != nil {
		return 0, false, err
	}
	t, ok := tonnes[strings.ToUpper(country)]
	if !ok {
		return 0, false, nil
	}
	em := ember.New(key)
	em.Allow = func(ctx context.Context) error { return a.meter.Allow(ctx, "ember") }
	em.Done = func(ctx context.Context, failed bool) { a.meter.Record(ctx, "ember", failed) }
	gen, err := em.GenerationTWh(ctx, []string{country}, year)
	if err != nil {
		return 0, false, err
	}
	twh, ok := gen[strings.ToUpper(country)]
	if !ok {
		return 0, false, nil
	}
	g := t / (twh * 1000)
	if g < 1 || g > 1500 {
		return 0, false, nil
	}
	return g, true, nil
}
