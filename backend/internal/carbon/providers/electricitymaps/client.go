// Package electricitymaps implements domain.CarbonDataProvider on the
// Electricity Maps HTTP API.
package electricitymaps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
)

// defaultZones maps AWS regions to Electricity Maps zone keys. Zone keys are provider data that can change
// or differ per subscription, so the map is verified against the account's available zones by
// `admin doctor` and can be corrected without a release via ELECTRICITYMAPS_ZONE_OVERRIDES.
var defaultZones = map[string]string{
	"us-east-1": "US-MIDA-PJM", "us-east-2": "US-MIDW-MISO", "us-west-1": "US-CAL-CISO", "us-west-2": "US-NW-PACW",
	"ca-central-1": "CA-QC", "ca-west-1": "CA-AB",
	"eu-west-1": "IE", "eu-west-2": "GB", "eu-west-3": "FR", "eu-central-1": "DE", "eu-central-2": "CH",
	"eu-north-1": "SE", "eu-south-1": "IT-NO", "eu-south-2": "ES",
	"ap-northeast-1": "JP-TK", "ap-northeast-2": "KR", "ap-northeast-3": "JP-KN", "ap-southeast-1": "SG",
	"ap-southeast-2": "AU-NSW", "ap-southeast-4": "AU-VIC", "ap-south-1": "IN-WE", "ap-east-1": "HK",
	"sa-east-1": "BR-S", "il-central-1": "IL", "af-south-1": "ZA", "me-south-1": "BH",
}

var (
	regionKeyRe = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)
	zoneKeyRe   = regexp.MustCompile(`^[A-Z]{2}(-[A-Z0-9]+)*$`)
)

// ParseOverrides parses "region=ZONE,region=ZONE". Both sides are validated: the values end up in an outbound
// request URL, so nothing outside the expected alphabets is ever accepted.
func ParseOverrides(spec string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		region, zone, ok := strings.Cut(part, "=")
		region, zone = strings.TrimSpace(region), strings.TrimSpace(zone)
		if !ok || !regionKeyRe.MatchString(region) || !zoneKeyRe.MatchString(zone) || len(zone) > 20 {
			return nil, fmt.Errorf("electricitymaps: bad zone override %q (want region=ZONE, e.g. eu-central-1=DE)", part)
		}
		out[region] = zone
	}
	return out, nil
}

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	zones   map[string]string

	// Responses are cached so dashboards and jobs do not hit the provider API on every request.
	// Keys come from the fixed region -> zone map, so the cache cannot grow without bound.
	mu    sync.Mutex
	cache map[string]cacheEntry
	now   func() time.Time // replaced in tests
}

// Cache lifetimes: the latest reading changes hourly; a forecast is refreshed about once per hour.
const (
	intensityTTL = 15 * time.Minute
	forecastTTL  = time.Hour
)

type cacheEntry struct {
	value   any
	expires time.Time
}

// New builds a client. overrides (may be nil) replace or extend the default region -> zone map.
func New(apiKey string, overrides map[string]string) *Client {
	z := make(map[string]string, len(defaultZones)+len(overrides))
	for k, v := range defaultZones {
		z[k] = v
	}
	for k, v := range overrides {
		z[k] = v
	}
	return &Client{BaseURL: "https://api.electricitymaps.com/v3", APIKey: apiKey, zones: z, cache: map[string]cacheEntry{}, now: time.Now,
		HTTP: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// ZoneMap returns a copy of the effective region -> zone map.
func (c *Client) ZoneMap() map[string]string {
	out := make(map[string]string, len(c.zones))
	for k, v := range c.zones {
		out[k] = v
	}
	return out
}

// Zones lists the zone keys available to this API key (used by `admin doctor` to validate the map).
func (c *Client) Zones(ctx context.Context) (map[string]struct{}, error) {
	var body map[string]json.RawMessage
	if err := c.get(ctx, "/zones", &body); err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(body))
	for k := range body {
		out[k] = struct{}{}
	}
	return out, nil
}

// Regions lists the cloud regions this client can resolve to a zone.
func (c *Client) Regions() []string { return sortedRegions(c.zones) }

func sortedRegions(zones map[string]string) []string {
	out := make([]string, 0, len(zones))
	for r := range zones {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// cached returns a fresh cached value for key or computes it with load. Only successful results are cached,
// so a provider outage is retried on the next call instead of being remembered.
func cached[T any](c *Client, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	c.mu.Lock()
	if e, ok := c.cache[key]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.value.(T), nil
	}
	c.mu.Unlock()

	v, err := load()
	if err != nil {
		var zero T
		return zero, err
	}
	c.mu.Lock()
	c.cache[key] = cacheEntry{value: v, expires: c.now().Add(ttl)}
	c.mu.Unlock()
	return v, nil
}

func (c *Client) GetIntensity(ctx context.Context, region string) (domain.Intensity, error) {
	zone, ok := c.zones[region]
	if !ok {
		return domain.Intensity{}, fmt.Errorf("electricitymaps: no zone for region %q", region)
	}
	return cached(c, "latest:"+zone, intensityTTL, func() (domain.Intensity, error) { return c.fetchIntensity(ctx, region, zone) })
}

func (c *Client) fetchIntensity(ctx context.Context, region, zone string) (domain.Intensity, error) {
	var body struct {
		CarbonIntensity float64   `json:"carbonIntensity"`
		Datetime        time.Time `json:"datetime"`
	}
	if err := c.get(ctx, "/carbon-intensity/latest?zone="+url.QueryEscape(zone), &body); err != nil {
		return domain.Intensity{}, err
	}
	return domain.Intensity{Region: region, GPerKWh: body.CarbonIntensity, At: body.Datetime}, nil
}

func (c *Client) GetForecast(ctx context.Context, region string) (domain.Forecast, error) {
	zone, ok := c.zones[region]
	if !ok {
		return domain.Forecast{}, fmt.Errorf("electricitymaps: no zone for region %q", region)
	}
	return cached(c, "forecast:"+zone, forecastTTL, func() (domain.Forecast, error) { return c.fetchForecast(ctx, region, zone) })
}

func (c *Client) fetchForecast(ctx context.Context, region, zone string) (domain.Forecast, error) {
	var body struct {
		Forecast []struct {
			CarbonIntensity float64   `json:"carbonIntensity"`
			Datetime        time.Time `json:"datetime"`
		} `json:"forecast"`
	}
	if err := c.get(ctx, "/carbon-intensity/forecast?zone="+url.QueryEscape(zone), &body); err != nil {
		return domain.Forecast{}, err
	}
	f := domain.Forecast{Region: region}
	for _, p := range body.Forecast {
		f.Points = append(f.Points, domain.Intensity{Region: region, GPerKWh: p.CarbonIntensity, At: p.Datetime, IsForecast: true})
	}
	return f, nil
}

func (c *Client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("auth-token", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("electricitymaps: status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(dst)
}

// Regions lists the same regions as the real client (development only).
func (Static) Regions() []string { return sortedRegions(defaultZones) }

// Static is a fixed-intensity provider for local development only.
type Static struct{ GPerKWh float64 }

func (s Static) GetIntensity(_ context.Context, region string) (domain.Intensity, error) {
	return domain.Intensity{Region: region, GPerKWh: s.GPerKWh, At: time.Now()}, nil
}
func (s Static) GetForecast(context.Context, string) (domain.Forecast, error) {
	return domain.Forecast{}, nil
}
