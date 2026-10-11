// Package ieso computes the carbon intensity of Ontario from the hourly generation by fuel type that the Independent
// Electricity System Operator publishes (https://reports-public.ieso.ca; public, no key, history from 2015).
//
// The figure is CALCULATED, not measured: intensity = sum(generation x emission factor) / sum(generation), with generic
// life-cycle factors per fuel (the same family the other computed sources use). Imports and exports are not accounted for.
// It is labelled as computed from the generation mix and ranks behind a source that publishes a measured intensity.
//
// The IESO publishes one XML file per year (the current year's is rewritten daily, about 6 MB), and times in Eastern Standard
// Time all year round (no daylight saving), as "hour ending": hour 1 is 00:00-01:00 EST.
package ieso

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Factors are gCO2e per kWh of generation by IESO fuel (lower-cased): generic life-cycle medians (IPCC AR5, WG III Annex III)
// for nuclear, gas, hydro, wind, solar PV and biomass; "other" is an assumption.
var Factors = map[string]float64{
	"nuclear": 12, "gas": 490, "hydro": 24, "wind": 11, "solar": 48, "biofuel": 230, "other": 450,
}

// EST is the time zone of IESO's reports: UTC-5 the whole year.
var EST = time.FixedZone("EST", -5*3600)

// Reading is one hour of computed intensity.
type Reading struct {
	At time.Time // start of the hour, UTC
	G  float64
}

type Client struct {
	BaseURL string // default https://reports-public.ieso.ca/public/GenOutputbyFuelHourly
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New() *Client {
	return &Client{BaseURL: "https://reports-public.ieso.ca/public/GenOutputbyFuelHourly",
		HTTP: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

type document struct {
	Days []struct {
		Day   string `xml:"Day"`
		Hours []struct {
			Hour  int `xml:"Hour"`
			Fuels []struct {
				Fuel   string  `xml:"Fuel"`
				Output float64 `xml:"EnergyValue>Output"`
			} `xml:"FuelTotal"`
		} `xml:"HourlyData"`
	} `xml:"DocBody>DailyData"`
}

// A year's file is big and changes once a day: it is kept for a while and asked again conditionally.
type cached struct {
	readings       []Reading
	etag, modified string
	fetched        time.Time
}

var (
	mu    sync.Mutex
	years = map[int]*cached{}
)

// Hourly returns the computed intensity of Ontario for [from, to). Hours without generation are skipped.
func (c *Client) Hourly(ctx context.Context, from, to time.Time) ([]Reading, error) {
	var out []Reading
	for y := from.In(EST).Year(); y <= to.Add(-time.Second).In(EST).Year(); y++ {
		rs, err := c.year(ctx, y)
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			if !r.At.Before(from) && r.At.Before(to) {
				out = append(out, r)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

func (c *Client) year(ctx context.Context, y int) (rs []Reading, err error) {
	mu.Lock()
	hit := years[y]
	mu.Unlock()
	ttl := 30 * time.Minute
	if y < time.Now().In(EST).Year() {
		ttl = 24 * time.Hour // a past year does not change
	}
	if hit != nil && time.Since(hit.fetched) < ttl {
		return hit.readings, nil
	}
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return nil, fmt.Errorf("ieso: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/PUB_GenOutputbyFuelHourly_%d.xml", c.BaseURL, y), nil)
	if err != nil {
		return nil, err
	}
	if hit != nil {
		if hit.etag != "" {
			req.Header.Set("If-None-Match", hit.etag)
		}
		if hit.modified != "" {
			req.Header.Set("If-Modified-Since", hit.modified)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("ieso: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusNotModified:
		mu.Lock()
		hit.fetched = time.Now()
		mu.Unlock()
		return hit.readings, nil
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, nil // a year with no report yet
	default:
		return nil, fmt.Errorf("ieso: status %d", resp.StatusCode)
	}
	rs, err = Parse(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	mu.Lock()
	years[y] = &cached{readings: rs, etag: resp.Header.Get("ETag"), modified: resp.Header.Get("Last-Modified"), fetched: time.Now()}
	mu.Unlock()
	return rs, nil
}

// Parse turns one year's report into hourly intensities.
func Parse(r io.Reader) ([]Reading, error) {
	var d document
	if err := xml.NewDecoder(r).Decode(&d); err != nil {
		return nil, fmt.Errorf("ieso: unreadable report: %w", err)
	}
	var out []Reading
	for _, day := range d.Days {
		date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(day.Day), EST)
		if err != nil {
			continue
		}
		for _, h := range day.Hours {
			if h.Hour < 1 || h.Hour > 24 {
				continue
			}
			var all, weighted float64
			for _, f := range h.Fuels {
				if f.Output <= 0 || math.IsNaN(f.Output) {
					continue
				}
				fac, ok := Factors[strings.ToLower(strings.TrimSpace(f.Fuel))]
				if !ok {
					fac = Factors["other"]
				}
				all += f.Output
				weighted += f.Output * fac
			}
			if all <= 0 {
				continue
			}
			g := weighted / all
			if g < 1 || g > 1500 {
				continue
			}
			out = append(out, Reading{At: date.Add(time.Duration(h.Hour-1) * time.Hour).UTC(), G: g})
		}
	}
	return out, nil
}
