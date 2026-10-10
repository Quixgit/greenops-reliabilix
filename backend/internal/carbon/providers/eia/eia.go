// Package eia computes the carbon intensity of a United States balancing authority from the hourly net generation by
// fuel type published by the U.S. Energy Information Administration (EIA-930, Open Data API v2; free with a key).
//
// The figure is CALCULATED, not measured: intensity = sum(generation x emission factor) / sum(generation), with
// generic life-cycle factors per fuel (below). It is labelled as computed from the generation mix and ranks behind a
// source that publishes a measured intensity. Imports and exports between balancing authorities are not accounted for.
package eia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Factors are gCO2e per kWh of net generation by EIA fuel type code. They are generic life-cycle medians (IPCC AR5,
// WG III Annex III, for coal, gas, biomass, solar PV, geothermal, hydro, nuclear, wind) and a typical combustion value for
// petroleum; "OTH" (other, unknown) is an assumption. They are approximations for an estimate, not a certified inventory.
var Factors = map[string]float64{
	"COL": 820, // coal
	"NG":  490, // natural gas
	"OIL": 650, // petroleum (typical combustion value; AR5 has no median)
	"NUC": 12,  // nuclear
	"WAT": 24,  // hydro
	"WND": 11,  // wind
	"SUN": 48,  // solar (utility-scale PV)
	"GEO": 38,  // geothermal
	"BIO": 230, // biomass
	"OTH": 450, // other: assumed, between gas and the clean sources
}

// storage flows are neither generation nor emitting here: they are left out of the mix.
var skip = map[string]bool{"BAT": true, "PS": true, "ESS": true}

// Respondents maps an Electricity Maps zone (the platform's region to zone map) to the EIA balancing authority code.
var Respondents = map[string]string{
	"US-MIDA-PJM": "PJM", "US-MIDW-MISO": "MISO", "US-CAL-CISO": "CISO", "US-NW-PACW": "PACW", "US-NW-PACE": "PACE",
	"US-TEX-ERCO": "ERCO", "US-CAR-SC": "SC", "US-NY-NYIS": "NYIS", "US-NE-ISNE": "ISNE", "US-SW-AZPS": "AZPS",
}

// Reading is one hour of computed intensity.
type Reading struct {
	At time.Time // start of the hour, UTC
	G  float64
}

type Client struct {
	BaseURL string // default https://api.eia.gov
	APIKey  string
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New(key string) *Client {
	return &Client{BaseURL: "https://api.eia.gov", APIKey: key,
		HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

const (
	pageSize  = 5000
	chunk     = 14 * 24 * time.Hour // 14 days x 24 h x ~9 fuels stays inside one page
	minShare  = 0.8                 // a hour is used only when this share of its generation has a known factor
	periodFmt = "2006-01-02T15"
)

var errNoKey = errors.New("eia: no API key is configured")

// Hourly returns the computed intensity of a balancing authority for [from, to). Hours without positive generation are
// skipped; so are hours where the fuels with a known factor are less than 80% of the generation.
func (c *Client) Hourly(ctx context.Context, respondent string, from, to time.Time) ([]Reading, error) {
	if c.APIKey == "" {
		return nil, errNoKey
	}
	var out []Reading
	for start := from.UTC().Truncate(time.Hour); start.Before(to); start = start.Add(chunk) {
		end := start.Add(chunk)
		if end.After(to) {
			end = to
		}
		rows, err := c.fetch(ctx, respondent, start, end)
		if err != nil {
			return nil, err
		}
		out = append(out, mix(rows)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

type row struct {
	period   time.Time
	fueltype string
	mwh      float64
}

func (c *Client) fetch(ctx context.Context, respondent string, from, to time.Time) ([]row, error) {
	var out []row
	for offset := 0; ; offset += pageSize {
		q := url.Values{
			"api_key": {c.APIKey}, "frequency": {"hourly"}, "data[0]": {"value"}, "facets[respondent][]": {respondent},
			"start": {from.Format(periodFmt)}, "end": {to.Add(-time.Hour).Format(periodFmt)},
			"sort[0][column]": {"period"}, "sort[0][direction]": {"asc"}, "length": {strconv.Itoa(pageSize)}, "offset": {strconv.Itoa(offset)},
		}
		var body struct {
			Response struct {
				Total json.Number `json:"total"`
				Data  []struct {
					Period   string          `json:"period"`
					Fueltype string          `json:"fueltype"`
					Value    json.RawMessage `json:"value"`
				} `json:"data"`
			} `json:"response"`
		}
		if err := c.get(ctx, "/v2/electricity/rto/fuel-type-data/data/?"+q.Encode(), &body); err != nil {
			return nil, err
		}
		for _, d := range body.Response.Data {
			p, err := time.Parse(periodFmt, d.Period)
			if err != nil {
				continue
			}
			v, ok := number(d.Value)
			if !ok {
				continue
			}
			out = append(out, row{period: p.UTC(), fueltype: d.Fueltype, mwh: v})
		}
		total, _ := strconv.Atoi(body.Response.Total.String())
		if len(body.Response.Data) < pageSize || offset+pageSize >= total {
			return out, nil
		}
		if offset > 20*pageSize { // a runaway paging loop must not hammer the API
			return out, errors.New("eia: too many pages")
		}
	}
}

// number reads a JSON number or a numeric string (EIA sends strings).
func number(raw json.RawMessage) (float64, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return f, err == nil && !math.IsNaN(f)
	}
	var f float64
	return f, json.Unmarshal(raw, &f) == nil && !math.IsNaN(f)
}

// mix turns rows of one balancing authority into hourly intensities. Negative values (solar at night, auxiliary load) and
// storage flows are not generation and are left out; an hour needs positive generation with 80% known factors.
func mix(rows []row) []Reading {
	type acc struct{ known, all, weighted float64 }
	by := map[time.Time]*acc{}
	for _, r := range rows {
		if skip[r.fueltype] || r.mwh <= 0 {
			continue
		}
		a := by[r.period]
		if a == nil {
			a = &acc{}
			by[r.period] = a
		}
		a.all += r.mwh
		f, ok := Factors[r.fueltype]
		if !ok {
			f, ok = Factors["OTH"], false // an unknown fuel counts as "other"
		}
		a.weighted += r.mwh * f
		if ok {
			a.known += r.mwh
		}
	}
	out := make([]Reading, 0, len(by))
	for p, a := range by {
		if a.all <= 0 || a.known/a.all < minShare {
			continue
		}
		g := a.weighted / a.all
		if g < 1 || g > 1500 {
			continue
		}
		out = append(out, Reading{At: p, G: g})
	}
	return out
}

// Check proves the key works with the smallest request; the key is never part of an error.
func (c *Client) Check(ctx context.Context) error {
	if c.APIKey == "" {
		return errNoKey
	}
	var body struct {
		Response struct {
			Data []json.RawMessage `json:"data"`
		} `json:"response"`
	}
	q := url.Values{"api_key": {c.APIKey}, "frequency": {"hourly"}, "data[0]": {"value"}, "facets[respondent][]": {"CISO"}, "length": {"1"}}
	if err := c.get(ctx, "/v2/electricity/rto/fuel-type-data/data/?"+q.Encode(), &body); err != nil {
		return err
	}
	if len(body.Response.Data) == 0 {
		return errors.New("eia: the key was accepted but returned no data")
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, dst any) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("eia: %w", err)
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
		return errors.New("eia: request failed") // the underlying error contains the URL and so the key
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(dst)
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("eia: the API key was not accepted")
	case http.StatusTooManyRequests:
		return errors.New("eia: rate limit reached, try later")
	}
	return fmt.Errorf("eia: status %d", resp.StatusCode)
}
