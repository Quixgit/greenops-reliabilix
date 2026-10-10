// Package ember reads monthly national electricity carbon intensity from the Ember data API (free with a key, CC-BY).
// It is a coarse, country-level, monthly source: no hourly readings, no sub-national zones. The platform uses it as a
// fallback behind an hourly source and for months where nothing finer exists, and labels it "month_mean".
package ember

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Reading is one month's national average, gCO2e per kWh.
type Reading struct {
	Month time.Time // first day of the month, UTC
	G     float64
}

type Client struct {
	BaseURL string // default https://api.ember-energy.org/v1
	APIKey  string
	HTTP    *http.Client
	// Allow and Done meter and cap requests (cost control); both optional.
	Allow func(ctx context.Context) error
	Done  func(ctx context.Context, failed bool)
}

func New(key string) *Client {
	return &Client{BaseURL: "https://api.ember-energy.org/v1", APIKey: key,
		HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

const (
	minIntensity = 1.0
	maxIntensity = 2000.0
)

// iso3 maps the country part of an Electricity Maps zone ("DE", "US-CAL-CISO" -> "US") to Ember's ISO 3166-1 alpha-3
// entity code. Only countries the platform's region map can produce are listed.
var iso3 = map[string]string{
	"US": "USA", "CA": "CAN", "IE": "IRL", "GB": "GBR", "FR": "FRA", "DE": "DEU", "CH": "CHE", "SE": "SWE", "IT": "ITA", "ES": "ESP",
	"JP": "JPN", "KR": "KOR", "SG": "SGP", "AU": "AUS", "IN": "IND", "HK": "HKG", "BR": "BRA", "IL": "ISR", "ZA": "ZAF", "BH": "BHR",
	"FI": "FIN", "BE": "BEL", "NL": "NLD", "PL": "POL", "TW": "TWN", "CL": "CHL", "QA": "QAT", "NO": "NOR", "DK": "DNK", "AT": "AUT",
}

// CountryOf returns Ember's entity code for an Electricity Maps zone, ok=false when the country is not covered.
func CountryOf(zone string) (string, bool) {
	c := strings.ToUpper(strings.SplitN(zone, "-", 2)[0])
	code, ok := iso3[c]
	return code, ok
}

var errNoKey = errors.New("ember: no API key is configured")

// Monthly returns the monthly carbon intensity of a country for [from, to). Implausible values are dropped.
func (c *Client) Monthly(ctx context.Context, entityCode string, from, to time.Time) ([]Reading, error) {
	if c.APIKey == "" {
		return nil, errNoKey
	}
	q := url.Values{"entity_code": {entityCode}, "is_aggregate_entity": {"false"}, "start_date": {from.UTC().Format("2006-01")}, "end_date": {to.UTC().Format("2006-01")}, "api_key": {c.APIKey}}
	var body struct {
		Data []struct {
			Date      string  `json:"date"`
			Intensity float64 `json:"emissions_intensity_gco2_per_kwh"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/carbon-intensity/monthly?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	var out []Reading
	for _, d := range body.Data {
		m, err := time.Parse("2006-01", d.Date[:min(len(d.Date), 7)])
		if err != nil || math.IsNaN(d.Intensity) || d.Intensity < minIntensity || d.Intensity > maxIntensity {
			continue
		}
		out = append(out, Reading{Month: m.UTC(), G: d.Intensity})
	}
	return out, nil
}

// Check proves the key works with the smallest request there is. The key is never part of an error message.
func (c *Client) Check(ctx context.Context) error {
	if c.APIKey == "" {
		return errNoKey
	}
	var body struct {
		Data []json.RawMessage `json:"data"`
	}
	q := url.Values{"entity_code": {"DEU"}, "start_date": {"2023"}, "end_date": {"2023"}, "api_key": {c.APIKey}}
	if err := c.get(ctx, "/carbon-intensity/yearly?"+q.Encode(), &body); err != nil {
		return err
	}
	if len(body.Data) == 0 {
		return errors.New("ember: the key was accepted but returned no data")
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, dst any) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("ember: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("ember: request failed") // the underlying error contains the URL and so the key
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(dst)
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("ember: the API key was not accepted")
	case http.StatusTooManyRequests:
		return errors.New("ember: rate limit reached, try later")
	}
	return fmt.Errorf("ember: status %d", resp.StatusCode)
}
