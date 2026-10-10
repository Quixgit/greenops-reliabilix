package electricitymaps

import (
	"context"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
)

// Dynamic is a provider whose API key and zone overrides are looked up on every use, so a change made in the
// operator console takes effect within the settings cache time, without a restart. The underlying Client is rebuilt
// only when the key or the overrides change, so its response cache survives between calls.
type Dynamic struct {
	// Source returns the current key and zone overrides ("" key = not configured).
	Source func(ctx context.Context) (apiKey, overrides string)
	// Allow and Done are passed to every client built (request cap and metering).
	Allow func(ctx context.Context) error
	Done  func(ctx context.Context, failed bool)
	// DevFallback serves a fixed intensity while no key is configured (development only).
	DevFallback bool

	mu      sync.Mutex
	client  *Client
	version string
}

func (d *Dynamic) current(ctx context.Context) (*Client, error) {
	key, spec := d.Source(ctx)
	if key == "" {
		return nil, domain.ErrNotConfigured
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if v := key + "\x00" + spec; d.client == nil || d.version != v {
		overrides, err := ParseOverrides(spec)
		if err != nil {
			return nil, err
		}
		d.client, d.version = New(key, overrides), v
		d.client.Allow, d.client.Done = d.Allow, d.Done
	}
	return d.client, nil
}

func (d *Dynamic) GetIntensity(ctx context.Context, region string) (domain.Intensity, error) {
	c, err := d.current(ctx)
	if err != nil {
		if d.DevFallback && err == domain.ErrNotConfigured {
			return Static{GPerKWh: 400}.GetIntensity(ctx, region)
		}
		return domain.Intensity{}, err
	}
	return c.GetIntensity(ctx, region)
}

func (d *Dynamic) GetForecast(ctx context.Context, region string) (domain.Forecast, error) {
	c, err := d.current(ctx)
	if err != nil {
		if d.DevFallback && err == domain.ErrNotConfigured {
			return domain.Forecast{}, nil
		}
		return domain.Forecast{}, err
	}
	return c.GetForecast(ctx, region)
}

// Regions implements domain.RegionLister: none while no key is configured, so the hourly refresh does nothing.
func (d *Dynamic) Regions() []string {
	c, err := d.current(context.Background())
	if err != nil {
		return nil
	}
	return c.Regions()
}

// GetHistory delegates to the current client; it fails closed when no key is configured (no invented history).
func (d *Dynamic) GetHistory(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	c, err := d.current(ctx)
	if err != nil {
		return nil, err
	}
	return c.GetHistory(ctx, region, from, to)
}
