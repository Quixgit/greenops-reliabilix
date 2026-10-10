// Package energycharts computes the carbon intensity of a European country from the public electricity generation by
// production type published on Energy-Charts (Fraunhofer ISE, https://api.energy-charts.info; free, no key, CC BY 4.0).
// The underlying data comes from the transmission system operators (ENTSO-E and national sources).
//
// The figure is CALCULATED, not measured: intensity = sum(generation x emission factor) / sum(generation), with generic
// life-cycle factors per production type. It is labelled as computed from the generation mix and ranks behind a source
// that publishes a measured intensity. Cross-border flows are not accounted for, and pumped storage is left out.
package energycharts

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
	"strings"
	"time"
)

// Factors are gCO2e per kWh of generation by Energy-Charts production type (lower-cased). They are generic life-cycle
// medians (IPCC AR5, WG III Annex III) for coal, gas, biomass, solar PV, geothermal, hydro, nuclear and wind; lignite,
// oil, derived gas, waste and "others" are assumptions. They are approximations for an estimate, not an inventory.
var Factors = map[string]float64{
	"fossil brown coal / lignite": 1000, // assumption: lignite is dirtier than the AR5 coal median (820)
	"fossil hard coal":            820,
	"fossil gas":                  490,
	"fossil oil":                  650,
	"fossil coal-derived gas":     500, // assumption
	"nuclear":                     12,
	"hydro run-of-river":          24,
	"hydro water reservoir":       24,
	"wind onshore":                11,
	"wind offshore":               12,
	"solar":                       48,
	"geothermal":                  38,
	"biomass":                     230,
	"waste":                       450, // assumption: part biogenic, part fossil
	"others":                      450, // assumption
}

// Countries are the Energy-Charts country codes this connector asks for, keyed by the first part of a zone key.
var Countries = map[string]string{
	"DE": "de", "FR": "fr", "ES": "es", "IT": "it", "PL": "pl", "SE": "se", "FI": "fi", "BE": "be", "NL": "nl",
	"CH": "ch", "AT": "at", "NO": "no", "DK": "dk", "PT": "pt", "CZ": "cz", "HU": "hu", "GR": "gr", "IE": "ie",
}

// Reading is one hour of computed intensity.
type Reading struct {
	At time.Time // start of the hour, UTC
	G  float64
}

type Client struct {
	BaseURL string // default https://api.energy-charts.info
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New() *Client {
	return &Client{BaseURL: "https://api.energy-charts.info",
		HTTP: &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

const (
	chunk    = 31 * 24 * time.Hour // a month of quarter-hours is about 3000 points per series
	minShare = 0.8                 // an hour is used only when this share of its generation has a known factor
)

type series struct {
	Name string     `json:"name"`
	Data []*float64 `json:"data"`
}

type body struct {
	UnixSeconds     []int64  `json:"unix_seconds"`
	ProductionTypes []series `json:"production_types"`
}

// Hourly returns the computed intensity of a country for [from, to). Hours without generation, or where the types with a
// known factor are under 80% of it, are skipped.
func (c *Client) Hourly(ctx context.Context, country string, from, to time.Time) ([]Reading, error) {
	var out []Reading
	for start := from.UTC().Truncate(24 * time.Hour); start.Before(to); start = start.Add(chunk) {
		end := start.Add(chunk)
		if end.After(to) {
			end = to
		}
		var b body
		q := url.Values{"country": {country}, "start": {start.Format("2006-01-02")}, "end": {end.Add(-time.Second).Format("2006-01-02")}}
		if err := c.get(ctx, "/public_power?"+q.Encode(), &b); err != nil {
			return nil, err
		}
		out = append(out, mix(b)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// excluded names are not generation that emits here: demand, shares, trading and storage charging/discharging.
func excluded(name string) bool {
	n := strings.ToLower(name)
	for _, w := range []string{"load", "share", "trading", "consumption", "pumped", "battery", "storage"} {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

// mix turns the quarter-hour series into hourly intensities (generation-weighted over the hour).
func mix(b body) []Reading {
	type acc struct{ known, all, weighted float64 }
	by := map[time.Time]*acc{}
	for _, s := range b.ProductionTypes {
		if excluded(s.Name) {
			continue
		}
		f, known := Factors[strings.ToLower(s.Name)]
		if !known {
			f = Factors["others"]
		}
		for i, v := range s.Data {
			if v == nil || *v <= 0 || math.IsNaN(*v) || i >= len(b.UnixSeconds) {
				continue
			}
			h := time.Unix(b.UnixSeconds[i], 0).UTC().Truncate(time.Hour)
			a := by[h]
			if a == nil {
				a = &acc{}
				by[h] = a
			}
			a.all += *v
			a.weighted += *v * f
			if known {
				a.known += *v
			}
		}
	}
	out := make([]Reading, 0, len(by))
	for h, a := range by {
		if a.all <= 0 || a.known/a.all < minShare {
			continue
		}
		g := a.weighted / a.all
		if g < 1 || g > 1500 {
			continue
		}
		out = append(out, Reading{At: h, G: g})
	}
	return out
}

func (c *Client) get(ctx context.Context, path string, dst any) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("energy-charts: %w", err)
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
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("energy-charts: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(dst)
	case http.StatusTooManyRequests:
		return errors.New("energy-charts: rate limit reached, try later")
	}
	return fmt.Errorf("energy-charts: status %d", resp.StatusCode)
}
