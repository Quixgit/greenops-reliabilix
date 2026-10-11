package app

import (
	"context"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/ember"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// emberMonthly adapts the Ember client to the carbon module: a cloud region maps to its country through the same zone
// map the hourly provider uses (including the operator's zone overrides), and one request serves every region of a
// country within a refresh.
type emberMonthly struct {
	ints  *integrations.Store
	meter *apiusage.Meter

	mu    sync.Mutex
	cache map[string]cached // country + window -> rows, valid for ten minutes (one refresh run)
}

type cached struct {
	at   time.Time
	rows []ember.Reading
	err  error
}

// Countries is every country's monthly figure from Ember in one request, for the world map.
func (e *emberMonthly) Countries(ctx context.Context, from, to time.Time) ([]domain.CountryReading, error) {
	key := e.ints.Value(ctx, "ember", "api_key")
	if key == "" {
		return nil, domain.ErrNotConfigured
	}
	cl := ember.New(key)
	cl.Allow = func(ctx context.Context) error { return e.meter.Allow(ctx, "ember") }
	cl.Done = func(ctx context.Context, failed bool) { e.meter.Record(ctx, "ember", failed) }
	rows, err := cl.MonthlyAll(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CountryReading, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.CountryReading{Code: r.Code, Month: r.Month, G: r.G})
	}
	return out, nil
}

func (e *emberMonthly) Monthly(ctx context.Context, region string, from, to time.Time) ([]domain.MonthlyRow, error) {
	key := e.ints.Value(ctx, "ember", "api_key")
	if key == "" {
		return nil, domain.ErrNotConfigured
	}
	zones := electricitymaps.DefaultZones()
	if ov, err := electricitymaps.ParseOverrides(e.ints.Value(ctx, "electricitymaps", "zone_overrides")); err == nil {
		for k, v := range ov {
			zones[k] = v
		}
	}
	country, ok := ember.CountryOf(zones[region])
	if !ok {
		return nil, nil // a region whose country Ember does not cover simply has no monthly figure
	}
	ck := country + from.Format("2006-01") + to.Format("2006-01")
	e.mu.Lock()
	if e.cache == nil {
		e.cache = map[string]cached{}
	}
	c, hit := e.cache[ck]
	e.mu.Unlock()
	if !hit || time.Since(c.at) > 10*time.Minute {
		cl := ember.New(key)
		cl.Allow = func(ctx context.Context) error { return e.meter.Allow(ctx, "ember") }
		cl.Done = func(ctx context.Context, failed bool) { e.meter.Record(ctx, "ember", failed) }
		rows, err := cl.Monthly(ctx, country, from, to)
		c = cached{at: time.Now(), rows: rows, err: err}
		e.mu.Lock()
		e.cache[ck] = c
		e.mu.Unlock()
	}
	if c.err != nil {
		return nil, c.err
	}
	out := make([]domain.MonthlyRow, 0, len(c.rows))
	for _, r := range c.rows {
		out = append(out, domain.MonthlyRow{Month: r.Month, Provider: domain.ProviderEmber, G: r.G})
	}
	return out, nil
}
