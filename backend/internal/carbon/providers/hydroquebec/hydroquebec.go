// Package hydroquebec computes the carbon intensity of Quebec from the generation mix that Hydro-Quebec publishes as open data
// (https://donnees.hydroquebec.com; public, no key). Two datasets are used: the live one (the last two days, hourly) and the
// history (hourly, 2019 to the end of 2025, rewritten once a year).
//
// The figure is CALCULATED, not measured: intensity = sum(generation x emission factor) / sum(generation), with generic
// life-cycle factors per source. It is Quebec's own generation: imports are not accounted for. It is labelled as computed
// from the generation mix and ranks behind a source that publishes a measured intensity.
package hydroquebec

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
	"time"
)

// Factors are gCO2e per kWh of generation by Hydro-Quebec source: generic life-cycle medians (IPCC AR5, WG III Annex III) for
// hydro, wind and solar PV; thermal (oil-fired peaking plants) and "autres" (mostly biomass and cogeneration) are assumptions.
var Factors = map[string]float64{"hydraulique": 24, "eolien": 11, "solaire": 48, "thermique": 650, "autres": 230}

// Reading is one hour of computed intensity.
type Reading struct {
	At time.Time // start of the hour, UTC
	G  float64
}

type Client struct {
	BaseURL string // default https://donnees.hydroquebec.com/api/explore/v2.1/catalog/datasets
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New() *Client {
	return &Client{BaseURL: "https://donnees.hydroquebec.com/api/explore/v2.1/catalog/datasets",
		HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// The history ends here; from this instant on only the live dataset has data.
var historyEnd = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

type live struct {
	Date      time.Time `json:"date"`
	Hydraulic *float64  `json:"valeurs_hydraulique"`
	Wind      *float64  `json:"valeurs_eolien"`
	Others    *float64  `json:"valeurs_autres"`
	Solar     *float64  `json:"valeurs_solaire"`
	Thermal   *float64  `json:"valeurs_thermique"`
}

type history struct {
	Date      time.Time `json:"date"`
	Hydraulic *float64  `json:"hydraulique"`
	Wind      *float64  `json:"eolien"`
	Others    *float64  `json:"autres"`
	Solar     *float64  `json:"solaire"`
	Thermal   *float64  `json:"thermique"`
}

// Hourly returns the computed intensity of Quebec for [from, to). Where both datasets know an hour the live one wins.
func (c *Client) Hourly(ctx context.Context, from, to time.Time) ([]Reading, error) {
	by := map[time.Time]float64{}
	if from.Before(historyEnd) {
		var rows []history
		end := to
		if end.After(historyEnd) {
			end = historyEnd
		}
		if err := c.export(ctx, "historique-production-electricite-quebec", from, end, &rows); err != nil {
			return nil, err
		}
		for _, r := range rows {
			add(by, r.Date, r.Hydraulic, r.Wind, r.Others, r.Solar, r.Thermal)
		}
	}
	if to.After(time.Now().Add(-72 * time.Hour)) { // the live dataset holds the last two days
		var rows []live
		if err := c.export(ctx, "production-electricite-quebec", from, to, &rows); err != nil {
			return nil, err
		}
		for _, r := range rows {
			add(by, r.Date, r.Hydraulic, r.Wind, r.Others, r.Solar, r.Thermal)
		}
	}
	out := make([]Reading, 0, len(by))
	for h, g := range by {
		if !h.Before(from) && h.Before(to) {
			out = append(out, Reading{At: h, G: g})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// add turns one record into an hour's intensity. A record with no generation is a future hour: the live dataset lists them empty.
func add(by map[time.Time]float64, at time.Time, hyd, wind, oth, sol, th *float64) {
	var all, weighted float64
	for name, v := range map[string]*float64{"hydraulique": hyd, "eolien": wind, "autres": oth, "solaire": sol, "thermique": th} {
		if v == nil || *v <= 0 || math.IsNaN(*v) {
			continue
		}
		all += *v
		weighted += *v * Factors[name]
	}
	if all <= 0 {
		return
	}
	if g := weighted / all; g >= 1 && g <= 1500 {
		by[at.UTC().Truncate(time.Hour)] = g
	}
}

func (c *Client) export(ctx context.Context, dataset string, from, to time.Time, dst any) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("hydroquebec: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	q := url.Values{"where": {fmt.Sprintf(`date >= "%s" AND date < "%s"`, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))}, "order_by": {"date"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s/exports/json?%s", c.BaseURL, dataset, q.Encode()), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("hydroquebec: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hydroquebec: status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(dst)
}
