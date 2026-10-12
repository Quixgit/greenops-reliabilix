// Package watttime reads the MARGINAL grid emissions rate (MOER, signal `co2_moer`) from WattTime's API v3: the intensity of
// the generator that would ramp up or down if load changed, which is a different measure from the average intensity every
// other source reports. The two are never mixed: a value from here is returned as `Marginal` in g CO2e/kWh and the caller
// must store and show it under its own name (ADR-0049).
//
// The client is ready and tested against a stand-in server, but it is not wired into ingestion: the platform has no
// WattTime account yet, so nothing in the running system calls it. Units are read from the response (`meta.units`) and a
// unit the client does not know is an error, not a guess.
package watttime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Reading is one marginal-emissions point.
type Reading struct {
	At time.Time // UTC
	G  float64   // g CO2e per kWh, marginal
}

type Client struct {
	BaseURL  string // default https://api.watttime.org
	Username string
	Password string
	HTTP     *http.Client
	// Allow and Done meter and cap requests; both optional.
	Allow func(ctx context.Context) error
	Done  func(ctx context.Context, failed bool)

	mu      sync.Mutex
	token   string
	expires time.Time
	now     func() time.Time
}

func New(username, password string) *Client {
	return &Client{BaseURL: "https://api.watttime.org", Username: username, Password: password,
		HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

const (
	// tokenLife is shorter than the 30 minutes WattTime grants, so a token is never used at its edge.
	tokenLife = 25 * time.Minute
	// maxRange is what one historical request covers; longer ranges are requested in chunks.
	maxRange     = 30 * 24 * time.Hour
	maxBody      = 8 << 20
	minIntensity = 0.0
	maxIntensity = 3000.0 // g/kWh; the dirtiest marginal units are around 1,200
)

var (
	regionRe = regexp.MustCompile(`^[A-Z0-9_]{2,40}$`)

	ErrNoCredentials = errors.New("watttime: no user name or password configured")
	ErrCredentials   = errors.New("watttime: the user name or password was rejected")
	ErrNoAccess      = errors.New("watttime: the account has no access to this region or signal")
)

// gPerKWh converts a value in the unit WattTime states to g CO2e per kWh. 1 lb/MWh = 453.59237 g / 1000 kWh.
func gPerKWh(v float64, unit string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "lbs_co2_per_mwh", "lb_co2_per_mwh", "lbs/mwh", "lb/mwh":
		return v * 0.45359237, true
	case "kg_co2_per_mwh", "kg/mwh", "g_co2_per_kwh", "g/kwh":
		return v, true
	}
	return 0, false
}

func (c *Client) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// login returns a valid bearer token, logging in with HTTP Basic when there is none or it is about to expire.
func (c *Client) login(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && c.clock().Before(c.expires) {
		return c.token, nil
	}
	if c.Username == "" || c.Password == "" {
		return "", ErrNoCredentials
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/login", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Set("Accept", "application/json")
	var body struct {
		Token string `json:"token"`
	}
	if err := c.do(ctx, req, &body); err != nil {
		if errors.Is(err, errUnauthorized) || errors.Is(err, errForbidden) {
			return "", ErrCredentials
		}
		return "", err
	}
	if body.Token == "" {
		return "", errors.New("watttime: login returned no token")
	}
	c.token, c.expires = body.Token, c.clock().Add(tokenLife)
	return c.token, nil
}

var (
	errUnauthorized = errors.New("unauthorized")
	errForbidden    = errors.New("forbidden")
)

// do sends one request and decodes the JSON body. Errors never include the URL query, the credentials or the body.
func (c *Client) do(ctx context.Context, req *http.Request, dst any) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("watttime: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("watttime: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return errUnauthorized
	case http.StatusForbidden:
		return errForbidden
	case http.StatusTooManyRequests:
		return errors.New("watttime: rate limited")
	default:
		return fmt.Errorf("watttime: status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(dst)
}

// get calls an authorised endpoint; one expired token is replaced and the call repeated once.
func (c *Client) get(ctx context.Context, path string, q url.Values, dst any) error {
	for attempt := 0; ; attempt++ {
		tok, err := c.login(ctx, attempt > 0)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path+"?"+q.Encode(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		err = c.do(ctx, req, dst)
		switch {
		case errors.Is(err, errUnauthorized) && attempt == 0:
			continue
		case errors.Is(err, errUnauthorized):
			return ErrCredentials
		case errors.Is(err, errForbidden):
			return ErrNoAccess
		}
		return err
	}
}

// Check logs in and asks which signals and regions the account may read; it is what a "Test" button in the console runs.
func (c *Client) Check(ctx context.Context) error {
	var body json.RawMessage
	return c.get(ctx, "/v3/my-access", url.Values{}, &body)
}

// Region returns the balancing-authority region WattTime uses for a location (latitude, longitude in degrees).
func (c *Client) Region(ctx context.Context, lat, lon float64) (string, error) {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || math.IsNaN(lat) || math.IsNaN(lon) {
		return "", errors.New("watttime: coordinates out of range")
	}
	var body struct {
		Region string `json:"region"`
	}
	q := url.Values{"latitude": {fmt.Sprintf("%.5f", lat)}, "longitude": {fmt.Sprintf("%.5f", lon)}, "signal_type": {"co2_moer"}}
	if err := c.get(ctx, "/v3/region-from-loc", q, &body); err != nil {
		return "", err
	}
	if !regionRe.MatchString(body.Region) {
		return "", errors.New("watttime: no region for this location")
	}
	return body.Region, nil
}

type series struct {
	Data []struct {
		PointTime string   `json:"point_time"`
		Value     *float64 `json:"value"`
	} `json:"data"`
	Meta struct {
		Units string `json:"units"`
	} `json:"meta"`
}

// convert turns a response into readings inside [from, to): points without a value, with an unreadable time or an
// implausible value are skipped. A response whose unit is missing or unknown is refused whole.
func (s series) convert(from, to time.Time) ([]Reading, error) {
	var out []Reading
	for _, d := range s.Data {
		if d.Value == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, d.PointTime)
		if err != nil || t.Before(from) || !t.Before(to) {
			continue
		}
		g, ok := gPerKWh(*d.Value, s.Meta.Units)
		if !ok {
			return nil, fmt.Errorf("watttime: unknown unit %q", s.Meta.Units)
		}
		if math.IsNaN(g) || g < minIntensity || g > maxIntensity {
			continue
		}
		out = append(out, Reading{At: t.UTC(), G: g})
	}
	return out, nil
}

// Historical returns the measured marginal rate (5-minute points) in [from, to).
func (c *Client) Historical(ctx context.Context, region string, from, to time.Time) ([]Reading, error) {
	if !regionRe.MatchString(region) {
		return nil, errors.New("watttime: invalid region")
	}
	var out []Reading
	for start := from.UTC(); start.Before(to); start = start.Add(maxRange) {
		end := start.Add(maxRange)
		if end.After(to) {
			end = to
		}
		q := url.Values{"region": {region}, "signal_type": {"co2_moer"}, "start": {start.Format(time.RFC3339)}, "end": {end.UTC().Format(time.RFC3339)}}
		var s series
		if err := c.get(ctx, "/v3/historical", q, &s); err != nil {
			return nil, err
		}
		part, err := s.convert(from, to)
		if err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	return out, nil
}

// Forecast returns the forecast marginal rate for the next horizonHours (1-72), for planning only; it is never stored as a
// reading.
func (c *Client) Forecast(ctx context.Context, region string, horizonHours int) ([]Reading, error) {
	if !regionRe.MatchString(region) {
		return nil, errors.New("watttime: invalid region")
	}
	if horizonHours < 1 || horizonHours > 72 {
		return nil, errors.New("watttime: horizon must be 1-72 hours")
	}
	q := url.Values{"region": {region}, "signal_type": {"co2_moer"}, "horizon_hours": {fmt.Sprint(horizonHours)}}
	var s series
	if err := c.get(ctx, "/v3/forecast", q, &s); err != nil {
		return nil, err
	}
	return s.convert(time.Time{}, time.Now().Add(time.Duration(horizonHours+1)*time.Hour).UTC())
}
