// Package electricitymaps implements domain.CarbonDataProvider on the
// Electricity Maps HTTP API.
package electricitymaps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
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
	// Google Cloud regions (ids never collide with AWS ones). Like the AWS entries they are verified against the
	// account's available zones by `admin doctor` and can be corrected with ELECTRICITYMAPS_ZONE_OVERRIDES.
	"us-central1": "US-MIDW-MISO", "us-east1": "US-CAR-SC", "us-east4": "US-MIDA-PJM", "us-east5": "US-MIDA-PJM",
	"us-south1": "US-TEX-ERCO", "us-west1": "US-NW-PACW", "us-west2": "US-CAL-CISO", "us-west3": "US-NW-PACE",
	"northamerica-northeast1": "CA-QC", "northamerica-northeast2": "CA-ON",
	"southamerica-east1": "BR-S", "southamerica-west1": "CL-SEN",
	"europe-north1": "FI", "europe-west1": "BE", "europe-west2": "GB", "europe-west3": "DE", "europe-west4": "NL",
	"europe-west6": "CH", "europe-west8": "IT-NO", "europe-west9": "FR", "europe-west12": "IT-NO",
	"europe-southwest1": "ES", "europe-central2": "PL",
	"asia-east1": "TW", "asia-east2": "HK", "asia-northeast1": "JP-TK", "asia-northeast2": "JP-KN", "asia-northeast3": "KR",
	"asia-south1": "IN-WE", "asia-south2": "IN-NO", "asia-southeast1": "SG",
	"australia-southeast1": "AU-NSW", "australia-southeast2": "AU-VIC",
	"me-west1": "IL", "me-central1": "QA", "africa-south1": "ZA",
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
	// Allow is asked before every HTTP request (cost control: a daily cap); Done is told how each one ended. Both optional.
	Allow func(ctx context.Context) error
	Done  func(ctx context.Context, failed bool)

	mu       sync.Mutex
	cache    map[string]cacheEntry
	inflight map[string]*call
	now      func() time.Time // replaced in tests
	sleep    func(time.Duration)
}

// call is one in-flight load that concurrent callers of the same key wait for (no thundering herd on the provider).
type call struct {
	done chan struct{}
	val  any
	err  error
}

// Cache lifetimes: the latest reading changes hourly; a forecast is refreshed about once per hour.
const (
	intensityTTL = 15 * time.Minute
	forecastTTL  = time.Hour
)

type cacheEntry struct {
	value   any
	expires time.Time // after this the entry is stale: refreshed, but still served if the provider is down
	at      time.Time // when it was fetched
}

// staleAfterError is how long an old reading may stand in for a provider that cannot be reached. Grid intensity
// moves slowly; a 6-hour-old value is far better for a dashboard than an error, and the reading's own timestamp
// stays visible. Beyond this the error is returned: a day-old figure must not pass as current.
const staleAfterError = 6 * time.Hour

// Sanity bounds for a reading: no real grid is below 0 or above ~2000 gCO2e/kWh. A zero is also what an
// unpopulated response decodes to, which would silently turn into "this region is carbon free".
const (
	minIntensity  = 1.0
	maxIntensity  = 2000.0
	maxReadingAge = 6 * time.Hour
)

// New builds a client. overrides (may be nil) replace or extend the default region -> zone map.
func New(apiKey string, overrides map[string]string) *Client {
	z := make(map[string]string, len(defaultZones)+len(overrides))
	for k, v := range defaultZones {
		z[k] = v
	}
	for k, v := range overrides {
		z[k] = v
	}
	return &Client{BaseURL: "https://api.electricitymaps.com/v3", APIKey: apiKey, zones: z, cache: map[string]cacheEntry{}, inflight: map[string]*call{}, now: time.Now, sleep: time.Sleep,
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

// cached returns a fresh cached value for key or loads it. Concurrent callers share one load. When the load fails a
// stale value (at most staleAfterError old) is returned instead of the error; failures are never cached themselves.
func cached[T any](c *Client, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	c.mu.Lock()
	e, have := c.cache[key]
	if have && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.value.(T), nil
	}
	if cl, ok := c.inflight[key]; ok { // someone is already loading this key: wait for the same answer
		c.mu.Unlock()
		<-cl.done
		if cl.err != nil {
			return staleOr[T](c, key, cl.err)
		}
		return cl.val.(T), nil
	}
	cl := &call{done: make(chan struct{})}
	c.inflight[key] = cl
	c.mu.Unlock()

	v, err := load()

	c.mu.Lock()
	delete(c.inflight, key)
	if err == nil {
		c.cache[key] = cacheEntry{value: v, expires: c.now().Add(ttl), at: c.now()}
	}
	c.mu.Unlock()
	cl.val, cl.err = v, err
	close(cl.done)
	if err != nil {
		return staleOr[T](c, key, err)
	}
	return v, nil
}

func staleOr[T any](c *Client, key string, err error) (T, error) {
	c.mu.Lock()
	e, ok := c.cache[key]
	c.mu.Unlock()
	if ok && c.now().Sub(e.at) < staleAfterError {
		return e.value.(T), nil
	}
	var zero T
	return zero, err
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
	if err := c.checkReading(body.CarbonIntensity, body.Datetime, true); err != nil {
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
		if err := c.checkReading(p.CarbonIntensity, p.Datetime, false); err != nil {
			continue // one bad point must not drop the forecast; it is simply not used
		}
		f.Points = append(f.Points, domain.Intensity{Region: region, GPerKWh: p.CarbonIntensity, At: p.Datetime, IsForecast: true})
	}
	return f, nil
}

// historyChunk is the longest range one past-range request may cover at hourly granularity.
const historyChunk = 9 * 24 * time.Hour

// GetHistory returns the hourly readings of a region in [from, to), requested in chunks. Past data is a plan feature of
// the provider: a refusal comes back as an error that says so. Implausible points are dropped, never repaired.
func (c *Client) GetHistory(ctx context.Context, region string, from, to time.Time) ([]domain.Intensity, error) {
	zone, ok := c.zones[region]
	if !ok {
		return nil, fmt.Errorf("electricitymaps: no zone for region %q", region)
	}
	var out []domain.Intensity
	for start := from; start.Before(to); start = start.Add(historyChunk) {
		end := start.Add(historyChunk)
		if end.After(to) {
			end = to
		}
		var body struct {
			History []struct {
				CarbonIntensity float64   `json:"carbonIntensity"`
				Datetime        time.Time `json:"datetime"`
			} `json:"history"`
		}
		path := "/carbon-intensity/past-range?zone=" + url.QueryEscape(zone) + "&start=" + url.QueryEscape(start.UTC().Format(time.RFC3339)) + "&end=" + url.QueryEscape(end.UTC().Format(time.RFC3339))
		if err := c.get(ctx, path, &body); err != nil {
			var se statusError
			if errors.As(err, &se) && (se.status == http.StatusForbidden || se.status == http.StatusUnauthorized) {
				return nil, fmt.Errorf("electricitymaps: past data is not available on this key or plan (status %d)", se.status)
			}
			return nil, err
		}
		for _, p := range body.History {
			if c.checkReading(p.CarbonIntensity, p.Datetime, false) != nil || p.Datetime.Before(from) || !p.Datetime.Before(to) {
				continue
			}
			out = append(out, domain.Intensity{Region: region, GPerKWh: p.CarbonIntensity, At: p.Datetime, Measure: domain.MeasureAverage})
		}
	}
	return out, nil
}

// checkReading rejects values that cannot be real. current=true also refuses a reading that is too old to describe now.
func (c *Client) checkReading(g float64, at time.Time, current bool) error {
	switch {
	case math.IsNaN(g) || g < minIntensity || g > maxIntensity:
		return fmt.Errorf("electricitymaps: implausible carbon intensity %v", g)
	case at.IsZero():
		return fmt.Errorf("electricitymaps: reading without a timestamp")
	case current && c.now().Sub(at) > maxReadingAge:
		return fmt.Errorf("electricitymaps: the latest reading is %s old", c.now().Sub(at).Round(time.Minute))
	}
	return nil
}

const maxAttempts = 3

// get performs the request, retrying throttling (429), server errors (5xx) and network errors with a short, jittered
// backoff (honouring a small Retry-After). 4xx other than 429 are final: retrying an auth or plan problem only burns
// quota. Errors never contain the URL or the key.
func (c *Client) get(ctx context.Context, path string, dst any) error {
	var last error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			c.sleep(backoff(attempt, retryAfter(last)))
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		retry, err := c.getOnce(ctx, path, dst)
		if err == nil {
			return nil
		}
		if !retry {
			return err
		}
		last = err
	}
	return last
}

type statusError struct {
	status int
	wait   time.Duration
}

func (e statusError) Error() string { return fmt.Sprintf("electricitymaps: status %d", e.status) }

func retryAfter(err error) time.Duration {
	var se statusError
	if errors.As(err, &se) {
		return se.wait
	}
	return 0
}

func backoff(attempt int, wait time.Duration) time.Duration {
	d := time.Duration(200<<uint(attempt-1)) * time.Millisecond // 200ms, 400ms
	d += time.Duration(rand.Int64N(int64(100 * time.Millisecond)))
	if wait > 0 && wait < 2*time.Second {
		d = max(d, wait)
	}
	return d
}

func (c *Client) getOnce(ctx context.Context, path string, dst any) (retry bool, err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return false, fmt.Errorf("electricitymaps: %w", err) // final: retrying would only ask again
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("auth-token", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return true, errors.New("electricitymaps: request failed") // the underlying error contains the URL
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusOK:
		return false, json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(dst)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		var wait time.Duration
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			wait = time.Duration(s) * time.Second
		}
		return true, statusError{status: resp.StatusCode, wait: wait}
	}
	return false, statusError{status: resp.StatusCode}
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
